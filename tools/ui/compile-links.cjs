// Compile the upstream Codex Link mixin supplied by update-codex.py on stdin.
const fs = require('node:fs');
const less = require('less');

less.render(fs.readFileSync(0, 'utf8'), { compress: true }).then(
    result => process.stdout.write(result.css + '\n'),
    error => { console.error(error); process.exitCode = 1; }
);
