// Preview already compiled H5 files; no dependency installation required.
const http = require('http')
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../page/unpackage/dist/build/h5')
const types = {'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.svg':'image/svg+xml','.png':'image/png','.ico':'image/x-icon'}
http.createServer((req,res)=>{
 let file
 try { file=path.resolve(root,'.'+decodeURIComponent(new URL(req.url,'http://localhost').pathname)) } catch {res.writeHead(400).end();return}
 if(file!==root&&!file.startsWith(root+path.sep)){res.writeHead(403).end();return}
 if(file===root)file=path.join(root,'index.html')
 fs.readFile(file,(err,body)=>{if(err){res.writeHead(404).end();return}res.writeHead(200,{'Content-Type':types[path.extname(file)]||'application/octet-stream'});res.end(body)})
}).listen(8081,'127.0.0.1',()=>console.log('H5 preview: http://localhost:8081'))
