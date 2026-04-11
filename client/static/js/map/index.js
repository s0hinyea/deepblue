import { renderGeoJsonLayer } from "./markers.js";

const DEFAULT_MAP_CENTER = [40.7128, -74.006];
const DEFAULT_MAP_ZOOM = 11;

export function createIncidentMap(elementId) {
  const mapElement = document.getElementById(elementId);

  if (!mapElement || typeof window.L === "undefined") {
    return null;
  }

  const map = window.L.map(elementId, {
    zoomControl: false,
  }).setView(DEFAULT_MAP_CENTER, DEFAULT_MAP_ZOOM);

  window.L.control
    .zoom({
      position: "bottomright",
    })
    .addTo(map);

  window.L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    attribution:
      '&copy; <a href="https://stadiamaps.com/" target="_blank" rel="noreferrer">Stadia Maps</a> ' +
      '&copy; <a href="https://openmaptiles.org/" target="_blank" rel="noreferrer">OpenMapTiles</a> ' +
      '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap</a>',
    maxZoom: 20,
    subdomains: "abcd",
  }).setUrl("https://tiles.stadiamaps.com/tiles/alidade_smooth_dark/{z}/{x}/{y}{r}.png").addTo(map);

  renderGeoJsonLayer(map);
  return map;
}
