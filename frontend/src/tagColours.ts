// Tags are colours only: a message is marked with one or more of a small fixed palette, shown as
// coloured dots (no names in the UI). Each palette colour maps to a stable tag id so applying the same
// colour always targets the same tag row through the existing tag storage.

export interface TagColour {
    colour: string
    name: string
}

// TAG_PALETTE is the fixed, curated set of tag colours, legible on both themes. The name is only used
// as a tooltip and for the underlying tag record; it is never shown as a label.
export const TAG_PALETTE: readonly TagColour[] = [
    {colour: '#e05252', name: 'Red'},
    {colour: '#e8833a', name: 'Orange'},
    {colour: '#d9a300', name: 'Amber'},
    {colour: '#4caf6e', name: 'Green'},
    {colour: '#2fb3a8', name: 'Teal'},
    {colour: '#3d7ff2', name: 'Blue'},
    {colour: '#7c5cff', name: 'Violet'},
    {colour: '#e05299', name: 'Pink'},
    {colour: '#a5744b', name: 'Brown'},
    {colour: '#8a94a6', name: 'Slate'},
]

// colourTagId derives a stable tag id from a palette colour.
export function colourTagId(colour: string): string {
    return 'colour-' + colour.replace('#', '')
}
