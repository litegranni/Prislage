/* Prisläge service worker — Web Push + offline shell */
self.addEventListener("install", (event) => {
  event.waitUntil(caches.open("prislage-v1").then((c) => c.addAll(["/", "/manifest.webmanifest"])));
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  event.respondWith(
    caches.match(req).then((cached) => cached || fetch(req).catch(() => caches.match("/")))
  );
});

self.addEventListener("push", (event) => {
  let data = { title: "Prisläge", body: "Nytt prislarm" };
  try {
    if (event.data) data = { ...data, ...event.data.json() };
  } catch (_) {}
  event.waitUntil(self.registration.showNotification(data.title, { body: data.body, icon: "/icon.svg" }));
});
