const SAMPLE_FEATURE_COLLECTION = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      geometry: {
        type: "Point",
        coordinates: [-73.969, 40.6602],
      },
      properties: {
        label: "High risk report",
        severity: "risk",
      },
    },
    {
      type: "Feature",
      geometry: {
        type: "Point",
        coordinates: [-73.9968, 40.7012],
      },
      properties: {
        label: "Watch area",
        severity: "watch",
      },
    },
    {
      type: "Feature",
      geometry: {
        type: "Point",
        coordinates: [-73.8196, 40.6155],
      },
      properties: {
        label: "Routine upload",
        severity: "safe",
      },
    },
  ],
};

const SEVERITY_COLORS = {
  risk: "#d94b4b",
  safe: "#2e9e6f",
  watch: "#e9a93b",
};

export function renderGeoJsonLayer(map) {
  if (!map || typeof window.L === "undefined") {
    return;
  }

  window.L.geoJSON(SAMPLE_FEATURE_COLLECTION, {
    pointToLayer(feature, latlng) {
      const severity = feature.properties?.severity ?? "watch";
      const markerColor = SEVERITY_COLORS[severity] ?? SEVERITY_COLORS.watch;

      return window.L.circleMarker(latlng, {
        color: markerColor,
        fillColor: markerColor,
        fillOpacity: 0.9,
        radius: 8,
        weight: 2,
      });
    },
    onEachFeature(feature, layer) {
      const label = feature.properties?.label;

      if (label) {
        layer.bindPopup(label);
      }
    },
  });
}
