const fs = require('fs');
const path = require('path');

function findApiFile(start) {
  let directory = path.resolve(start);
  while (true) {
    const candidate = path.join(directory, 'api.txt');
    if (fs.existsSync(candidate)) return candidate;
    const parent = path.dirname(directory);
    if (parent === directory) return null;
    directory = parent;
  }
}

function getTronGridApiKey() {
  if (process.env.TRONGRID_API_KEY) return process.env.TRONGRID_API_KEY;
  const apiFile = findApiFile(process.cwd());
  if (!apiFile) return '';
  const match = fs.readFileSync(apiFile, 'utf8').match(/^TRONGRID=(.+)$/m);
  return match ? match[1].trim() : '';
}

module.exports = { getTronGridApiKey };
