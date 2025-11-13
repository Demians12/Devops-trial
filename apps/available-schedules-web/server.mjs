import { createServer } from "http";
import { readFile } from "fs/promises";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.join(__dirname, "public");

const MIME_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml; charset=utf-8",
  ".png": "image/png",
  ".ico": "image/x-icon",
};

const PORT = Number(process.env.PORT || 3000);

async function serveStatic(urlPath) {
  let normalized = decodeURI(urlPath.split("?")[0]);
  if (normalized === "/") {
    normalized = "/index.html";
  }

  // Prevent path traversal
  normalized = normalized.replace(/\.\./g, "");

  const filePath = path.join(publicDir, normalized);
  const ext = path.extname(filePath).toLowerCase();
  const mime = MIME_TYPES[ext] || "application/octet-stream";
  const buffer = await readFile(filePath);
  return { buffer, mime };
}

const server = createServer(async (req, res) => {
  try {
    if (req.url === "/healthz") {
      res.writeHead(200, { "Content-Type": "application/json" });
      return res.end(JSON.stringify({ status: "ok" }));
    }

    const { buffer, mime } = await serveStatic(req.url);
    res.writeHead(200, {
      "Content-Type": mime,
      "Cache-Control": mime.startsWith("text/") ? "no-cache" : "public, max-age=604800",
    });
    res.end(buffer);
  } catch (error) {
    if (error.code === "ENOENT") {
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Not found" }));
    } else {
      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Internal server error" }));
    }
  }
});

server.listen(PORT, () => {
  console.log(`available-schedules-web listening on http://0.0.0.0:${PORT}`);
});

