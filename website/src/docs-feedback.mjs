/** @param {string} value */
export function shellQuote(value, shell = 'posix') {
  return shell === 'powershell'
    ? `'${value.replace(/['\u2018-\u201b]/g, (quote) => quote + quote)}'`
    : `'${value.replaceAll("'", "'\\''")}'`;
}

/** @param {{page: string, title: string, version?: string, section?: string, source?: string, quote: string, issue: string, summary: string}} report */
export function taskContent(report) {
  return {
    title: report.summary.trim() || `Docs: ${report.title}`,
    description: [
      `Page: ${report.page}`,
      report.section && `Section: ${report.section}`,
      report.version && `Documentation version: ${report.version}`,
      report.source && `Source: ${report.source}`,
      '',
      ...(report.quote.trim() ? ['Selected text:', ...report.quote.trim().split('\n').map((line) => `> ${line}`), ''] : []),
      'Issue:', report.issue.trim(),
    ].filter((line) => typeof line === 'string').join('\n'),
  };
}

/** @param {{title: string, description: string}} task */
export function taskURL(task) {
  const url = new URL('https://phabricator.wikimedia.org/maniphest/task/edit/form/1/');
  url.search = new URLSearchParams({ projects: 'a_wikimedia_cli', ...task }).toString();
  return url.href;
}

/** @param {{title: string, description: string}} task */
export function taskCommand(task, shell = 'posix') {
  if (shell === 'powershell') {
    const input = JSON.stringify({ ...task, projects: ['a_wikimedia_cli'] });
    // A temporary UTF-8 file avoids native argument quoting and leaves stdin free for confirmation.
    return `& { $mwDocsTaskFile = [System.IO.Path]::GetTempFileName(); try { [System.IO.File]::WriteAllText($mwDocsTaskFile, ${shellQuote(input, shell)}, [System.Text.UTF8Encoding]::new($false)); mw phab create --input $mwDocsTaskFile } finally { Remove-Item -LiteralPath $mwDocsTaskFile } }`;
  }
  return `mw phab create --title ${shellQuote(task.title, shell)} --project a_wikimedia_cli --description ${shellQuote(task.description, shell)}`;
}
