const fs = require('fs')
const path = require('path')

const sourceDir = path.join(__dirname, '..', 'node_modules', 'katex', 'dist')
const targetDir = path.join(__dirname, '..', 'public', 'vendor', 'katex')
const legacyMathJaxDir = path.join(__dirname, '..', 'public', 'vendor', 'mathjax')

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true })
  }
}

function copyFile(name) {
  const src = path.join(sourceDir, name)
  const dest = path.join(targetDir, name)
  ensureDir(path.dirname(dest))
  fs.copyFileSync(src, dest)
}

function copyFonts() {
  const sourceFonts = path.join(sourceDir, 'fonts')
  const targetFonts = path.join(targetDir, 'fonts')
  const entries = fs.readdirSync(sourceFonts)
  let i

  ensureDir(targetFonts)
  for (i = 0; i < entries.length; i += 1) {
    fs.copyFileSync(path.join(sourceFonts, entries[i]), path.join(targetFonts, entries[i]))
  }
}

ensureDir(targetDir)
if (fs.existsSync(legacyMathJaxDir)) {
  fs.rmSync(legacyMathJaxDir, { recursive: true, force: true })
}
copyFile('katex.min.css')
copyFonts()
