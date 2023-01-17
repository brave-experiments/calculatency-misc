package main

// LatencyTemplate represents a template for a CAPTCHA that assists a remote
// service in conducting application layer latency measurements.
const LatencyTemplate = `
<!doctype html>

<html lang="en">
  <head>
    <meta charset = "utf-8">
    <title>Solve the CAPTCHA</title>
  </head>

  <body>
    <p>Measurement status: <span id="status">Running</span></p>

    <script>
      function getLatencyWebSocket() {
        return new Promise(function(resolve, reject) {
          var socket = new WebSocket("{{.WebSocketEndpoint}}");
          socket.onerror = function (err) {
            console.log("Encountered WebSocket error: " + err.message);
            reject(err.toString());
          }

          socket.onopen = function() {
            console.log("WebSocket connection established.");
          }

          socket.onclose = function(event) {
            console.log("WebSocket connection closed.");
            resolve();
          }

          socket.onmessage = function(event) {
            socket.send(event.data);
            console.log("Echoed data: " + event.data);
          }
        });
      }

      getLatencyWebSocket().then(() => {
        document.getElementById("status").innerHTML = "Done";
      });
    </script>
  </body>
</html>
`
