// The product's name and mark live here only, so a rename touches one file.
// (Server-side texts and the Go module have their own names.)
export const BRAND = {
  name: 'Nefo',
  // The mark inside the rounded square: SVG path data on a 24×24 grid,
  // drawn with a 2.4px round stroke. Concept: Leap.
  mark: ['M5.6 17.7V11.7a5.2 5.2 0 0 1 10.4 0v2.6a3.4 3.4 0 0 0 3.4 3.4'],
}

// pageTitle sets the browser tab title: "Websites · Nefo".
export function setDocumentTitle(page) {
  document.title = page ? `${page} · ${BRAND.name}` : BRAND.name
}
