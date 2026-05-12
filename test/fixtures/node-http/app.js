const http = require('http');
const port = parseInt(process.env.TEST_NODE_APP_PORT || '18082', 10);
http.createServer((req, res) => {
  res.writeHead(200);
  res.end('OK');
}).listen(port, () => console.log('Listening on port ' + port));
