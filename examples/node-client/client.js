"use strict";

const http = require("node:http");
const https = require("node:https");
const tls = require("node:tls");

const target = new URL(process.env.AUTOCURL_DEMO_URL || "https://example.com/");
const proxyValue = target.protocol === "https:"
  ? process.env.HTTPS_PROXY || process.env.https_proxy
  : process.env.HTTP_PROXY || process.env.http_proxy;

request(target, proxyValue ? new URL(proxyValue) : undefined)
  .then(({ status, protocol, bytes }) => {
    console.log(`status=${status} protocol=${protocol} bytes=${bytes}`);
  })
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });

function request(url, proxy) {
  if (!proxy) {
    return requestDirect(url);
  }
  if (url.protocol === "http:") {
    return requestHTTPThroughProxy(url, proxy);
  }
  return requestHTTPSThroughProxy(url, proxy);
}

function requestDirect(url) {
  return new Promise((resolve, reject) => {
    const transport = url.protocol === "https:" ? https : http;
    const request = transport.get(url, {
      headers: {
        Connection: "close",
        "User-Agent": "autocurl-node-example/1",
      },
    }, (response) => consume(response, resolve, reject));
    request.setTimeout(15000, () => request.destroy(new Error("request timed out")));
    request.on("error", reject);
  });
}

function requestHTTPThroughProxy(url, proxy) {
  return new Promise((resolve, reject) => {
    const request = http.get({
      host: proxy.hostname,
      port: proxy.port || 80,
      path: url.href,
      headers: {
        Connection: "close",
        Host: url.host,
        "User-Agent": "autocurl-node-example/1",
      },
    }, (response) => consume(response, resolve, reject));
    request.setTimeout(15000, () => request.destroy(new Error("request timed out")));
    request.on("error", reject);
  });
}

function requestHTTPSThroughProxy(url, proxy) {
  return new Promise((resolve, reject) => {
    const connect = http.request({
      host: proxy.hostname,
      port: proxy.port || 80,
      method: "CONNECT",
      path: `${url.hostname}:${url.port || 443}`,
      headers: {Host: `${url.hostname}:${url.port || 443}`},
    });
    connect.setTimeout(15000, () => connect.destroy(new Error("proxy CONNECT timed out")));
    connect.on("connect", (response, socket, head) => {
      if (response.statusCode !== 200) {
        socket.destroy();
        reject(new Error(`proxy CONNECT returned ${response.statusCode}`));
        return;
      }
      if (head.length > 0) socket.unshift(head);
      const secureSocket = tls.connect({
        socket,
        servername: url.hostname,
        ALPNProtocols: ["http/1.1"],
      });
      secureSocket.once("error", reject);
      secureSocket.once("secureConnect", () => {
        const chunks = [];
        secureSocket.on("data", (chunk) => chunks.push(chunk));
        secureSocket.once("end", () => {
          const response = Buffer.concat(chunks);
          const headerEnd = response.indexOf("\r\n\r\n");
          const header = response.subarray(0, headerEnd < 0 ? response.length : headerEnd).toString();
          const match = /^HTTP\/(\S+)\s+(\d{3})/.exec(header);
          if (!match) {
            reject(new Error("upstream returned an invalid HTTP response"));
            return;
          }
          resolve({
            status: Number(match[2]),
            protocol: `HTTP/${match[1]}`,
            bytes: headerEnd < 0 ? 0 : response.length - headerEnd - 4,
          });
        });
        secureSocket.setTimeout(15000, () => secureSocket.destroy(new Error("request timed out")));
        secureSocket.write([
          `GET ${url.pathname}${url.search} HTTP/1.1`,
          `Host: ${url.host}`,
          "User-Agent: autocurl-node-example/1",
          "Connection: close",
          "",
          "",
        ].join("\r\n"));
      });
    });
    connect.on("error", reject);
    connect.end();
  });
}

function consume(response, resolve, reject) {
  let bytes = 0;
  response.on("data", (chunk) => {
    bytes += chunk.length;
  });
  response.on("end", () => {
    resolve({
      status: response.statusCode,
      protocol: `HTTP/${response.httpVersion}`,
      bytes,
    });
  });
  response.on("error", reject);
}
