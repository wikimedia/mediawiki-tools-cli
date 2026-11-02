$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object Text.UTF8Encoding
[Console]::OutputEncoding = New-Object Text.UTF8Encoding
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$code = (Get-Command code.cmd -ErrorAction SilentlyContinue).Source
if (!$code) {
    foreach ($candidate in @(
        "$env:LOCALAPPDATA\Programs\Microsoft VS Code\bin\code.cmd",
        "$env:ProgramFiles\Microsoft VS Code\bin\code.cmd"
    )) {
        if (Test-Path -LiteralPath $candidate) {
            $code = $candidate
            break
        }
    }
}
if (!$code) {
    throw 'Install VS Code for Windows before opening the remote editor.'
}
if ($request.locate) {
    [Console]::Write($env:LOCALAPPDATA)
    exit 0
}
$env:VSCODE_IPC_HOOK_CLI = $null
if ($request.launch) {
    # Launch the GUI without inheriting the CLI's stdio pipes. Otherwise a
    # PowerShell caller can remain blocked for the lifetime of the editor.
    $executable = Join-Path (Split-Path -Parent $code) '..\Code.exe'
    if (!(Test-Path -LiteralPath $executable)) {
        throw 'Could not locate Code.exe next to the Windows code launcher.'
    }
    $env:ELECTRON_RUN_AS_NODE = $null
    Start-Process -FilePath $executable -ArgumentList $request.commandLine
    exit 0
}
$arguments = @($request.arguments)
& $code @arguments
exit $LASTEXITCODE
