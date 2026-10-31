$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$reply = @{ version = 1 }
try {
    # Read at most one bounded UTF-8 JSON request. stdin is data, never code.
    $buffer = New-Object byte[] 16385
    $inputStream = [Console]::OpenStandardInput()
    $length = 0
    while ($length -lt $buffer.Length) {
        $count = $inputStream.Read($buffer, $length, $buffer.Length - $length)
        if ($count -eq 0) { break }
        $length += $count
    }
    if ($length -gt 16384) { throw 'invalid' }
    $json = [Text.Encoding]::UTF8.GetString($buffer, 0, $length)
    if (!$json.TrimStart().StartsWith('{')) { throw 'invalid' }
    try { $request = $json | ConvertFrom-Json } catch { throw 'invalid' }
    if ($request -isnot [PSCustomObject] -or ($request.version -isnot [int] -and $request.version -isnot [long]) -or $request.version -ne 1 -or $request.action -isnot [string]) { throw 'invalid' }
    $namespace = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__NAMESPACE_BASE64__'))
    $encodedService = 'secretstore.v1.' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($namespace)).TrimEnd('=').Replace('+', '-').Replace('/', '_')
    $service = $request.service
    $account = $request.account
    if ($request.action -cne 'list') {
        if ($service -isnot [string] -or $account -isnot [string] -or !$account -or ($service + $account).IndexOfAny([char[]]"`0`r`n") -ge 0) { throw 'invalid' }
        if (!$service.StartsWith($namespace + ':', [StringComparison]::Ordinal) -and $service -cne $encodedService) { throw 'invalid' }
    }
    if (@('get', 'set', 'delete', 'probe', 'list') -cnotcontains $request.action) { throw 'invalid' }
    $value = [byte[]]@()
    if ($request.action -ceq 'set') {
        if ($null -ne $request.value -and $request.value -isnot [string]) { throw 'invalid' }
        try { $value = [Convert]::FromBase64String([string]$request.value) } catch { throw 'invalid' }
        if ($value.Length -gt 2560 -or [Text.Encoding]::UTF8.GetByteCount($service) -ge 512) { throw 'too-large' }
    }
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Runtime.InteropServices;

public static class WindowsCredentials {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct Credential {
        public uint Flags, Type;
        public string TargetName, Comment;
        public long LastWritten;
        public uint BlobSize;
        public IntPtr Blob;
        public uint Persist, AttributeCount;
        public IntPtr Attributes;
        public string TargetAlias, UserName;
    }
    public sealed class Metadata {
        public string Service, Account, Target;
    }
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CredReadW(string target, uint type, uint flags, out IntPtr credential);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CredWriteW(ref Credential credential, uint flags);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CredDeleteW(string target, uint type, uint flags);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CredEnumerateW(string filter, uint flags, out uint count, out IntPtr credentials);
    [DllImport("advapi32.dll")]
    static extern void CredFree(IntPtr buffer);

    static void Check(bool success) {
        if (!success) throw new Win32Exception(Marshal.GetLastWin32Error());
    }
    static void Clear(IntPtr pointer, uint length) {
        for (uint i = 0; i < length; i++) Marshal.WriteByte(pointer, (int)i, 0);
    }
    public static byte[] Read(string target) {
        IntPtr pointer;
        Check(CredReadW(target, 1, 0, out pointer));
        Credential credential = new Credential();
        try {
            credential = (Credential)Marshal.PtrToStructure(pointer, typeof(Credential));
            if (credential.BlobSize > 2560) throw new InvalidOperationException();
            byte[] value = new byte[credential.BlobSize];
            if (value.Length > 0) Marshal.Copy(credential.Blob, value, 0, value.Length);
            return value;
        } finally {
            Clear(credential.Blob, credential.BlobSize);
            CredFree(pointer);
        }
    }
    public static void Write(string target, string account, byte[] value) {
        IntPtr pointer = Marshal.AllocHGlobal(Math.Max(1, value.Length));
        try {
            Marshal.Copy(value, 0, pointer, value.Length);
            Credential credential = new Credential();
            credential.Type = 1;
            credential.TargetName = target;
            credential.UserName = account;
            credential.Blob = pointer;
            credential.BlobSize = (uint)value.Length;
            credential.Persist = 2;
            Check(CredWriteW(ref credential, 0));
        } finally {
            Clear(pointer, (uint)value.Length);
            Marshal.FreeHGlobal(pointer);
        }
    }
    public static void Delete(string target) {
        Check(CredDeleteW(target, 1, 0));
    }
    public static Metadata[] List(string[] prefixes) {
        List<Metadata> result = new List<Metadata>();
        foreach (string prefix in prefixes) {
            uint count;
            IntPtr pointers;
            if (!CredEnumerateW(prefix + "*", 0, out count, out pointers)) {
                int error = Marshal.GetLastWin32Error();
                if (error == 1168) continue;
                throw new Win32Exception(error);
            }
            try {
                for (int i = 0; i < count; i++) {
                    IntPtr pointer = Marshal.ReadIntPtr(pointers, i * IntPtr.Size);
                    Credential credential = (Credential)Marshal.PtrToStructure(pointer, typeof(Credential));
                    // Enumeration supplies blobs; discard them without returning them.
                    Clear(credential.Blob, credential.BlobSize);
                    if (credential.Type != 1 || String.IsNullOrEmpty(credential.UserName) || !credential.TargetName.StartsWith(prefix, StringComparison.Ordinal)) continue;
                    string suffix = ":" + credential.UserName;
                    if (!credential.TargetName.EndsWith(suffix, StringComparison.Ordinal) || (credential.TargetName + credential.UserName).IndexOfAny(new char[] {'\0', '\r', '\n'}) >= 0) continue;
                    result.Add(new Metadata { Service = credential.TargetName.Substring(0, credential.TargetName.Length - suffix.Length), Account = credential.UserName, Target = credential.TargetName });
                }
            } finally { CredFree(pointers); }
        }
        return result.ToArray();
    }
}
'@
    $target = $service + ':' + $account
    switch -CaseSensitive ($request.action) {
        'get' { $data = [WindowsCredentials]::Read($target); try { $reply.value = [Convert]::ToBase64String($data) } finally { [Array]::Clear($data, 0, $data.Length) } }
        'set' { [WindowsCredentials]::Write($target, $account, $value) }
        'delete' { [WindowsCredentials]::Delete($target) }
        'probe' { $probe = [WindowsCredentials]::Read($service + ':availability'); [Array]::Clear($probe, 0, $probe.Length) }
        'list' {
            $reply.entries = @([WindowsCredentials]::List(@(($namespace + ':'), ($encodedService + ':'))) | ForEach-Object {
                @{ service = $_.Service; account = $_.Account; target = $_.Target }
            })
        }
    }
} catch {
    $reply = @{ version = 1; error = 'unavailable' }
    $exception = $_.Exception
    while ($exception.InnerException) { $exception = $exception.InnerException }
    if ($exception -is [ComponentModel.Win32Exception] -and $exception.NativeErrorCode -eq 1168) {
        if ($request.action -ceq 'probe') { $reply.Remove('error') } else { $reply.error = 'not-found' }
    } elseif ($exception.Message -ceq 'invalid' -or $exception.Message -ceq 'too-large') {
        $reply.error = $exception.Message
    }
} finally {
    if ($value) { [Array]::Clear($value, 0, $value.Length) }
    [Array]::Clear($buffer, 0, $buffer.Length)
}
[Console]::Out.Write(($reply | ConvertTo-Json -Depth 4 -Compress))
