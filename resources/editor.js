import CodeMirror from './codemirror/lib/codemirror'
import './codemirror/mode/gfm/gfm'
import './codemirror/mode/markdown/markdown'
import './codemirror/mode/xml/xml'
import './codemirror/mode/meta'
import './codemirror/addon/edit/continuelist'
import './codemirror/addon/mode/overlay'
import './codemirror/addon/hint/show-hint'

document.addEventListener('DOMContentLoaded', function () {
  let theme = 'base16-light'
  if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    theme = 'monokai'
  }
  let codeMirror = CodeMirror.fromTextArea(document.querySelector('textarea#content'), {
    theme: theme,
    mode: 'gfm',
    extraKeys: { 'Enter': 'newlineAndIndentContinueMarkdownList' },
    lineWrapping: true,
  })
  const uploadFile = (file) => {
    return new Promise(function (resolve, reject) {
      let folder = decodeURIComponent(document.location.pathname.replace(/^\/edit\//, '').replace(/[^\/]*$/, ''))
      let data = new FormData()
      data.append('file', file)
      data.append('name', folder + file.name)
      data.append('message', 'Adding file: ' + folder + file.name)

      return fetch('/wiki/upload', {
        method: 'POST',
        body: data
      })
        .then(response => {
          if (response.status === 204) {
            resolve(folder + file.name)
          } else {
            reject('status: ' + response.status)
          }
        })
        .catch(e => reject(e))
    })
  }
  const insertEmbedAtCursor = (editor, link) => {
    let doc = editor.getDoc()
    let cursor = doc.getCursor()
    doc.replaceRange('![[' + link + ']]', cursor)
  }
  codeMirror.on('drop', function (editor, e) {
    Array.prototype.forEach.call(e.dataTransfer.files, function (file) {
      uploadFile(file)
        .then(file => {
          insertEmbedAtCursor(editor, file)
        })
        .catch(e => console.log('Error uploading file: ' + e))
    })
    e.preventDefault()
  })

  codeMirror.on('inputRead', function (instance) {
    if (instance.state.completionActive) { return }
    let cur = instance.getCursor()
    let token = instance.getTokenAt(cur)
    if (token.string !== '[') { return }
    let previousToken = instance.getTokenAt(CodeMirror.Pos(cur.line, cur.ch - 1))
    if (previousToken.string !== '[') { return }
    let evenPreviouserToken = instance.getTokenAt(CodeMirror.Pos(cur.line, cur.ch - 2))
    const type = evenPreviouserToken.string === '!' ? 'file' : 'page'
    CodeMirror.commands.autocomplete(instance, autocomplete, {
      closeCharacters: /[|\]]/,
      completeSingle: false,
      type
    })
  })

  function autocomplete (editor, options) {
    let cur = editor.getCursor(), curLine = editor.getLine(cur.line)
    let end = cur.ch, start = end
    while (start && curLine.charAt(start - 1) !== '[') { start-- }
    let prefix = start !== end && curLine.slice(start, end).toLowerCase() || ''
    return fetch('/api/list?type=' + options.type)
      .then(response => response.json())
      .then(words => words.filter(x => x.startsWith(prefix)))
      .then(matches => {
        let res = { list: matches, from: CodeMirror.Pos(cur.line, start), to: CodeMirror.Pos(cur.line, end) }
        CodeMirror.on(res, 'pick', function () {
          let doc = editor.getDoc()
          let cursor = doc.getCursor()
          doc.replaceRange(']]', cursor)
        })
        return res
      })
  }
})
