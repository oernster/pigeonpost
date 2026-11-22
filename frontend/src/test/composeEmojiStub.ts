// composeEmojiStub stands in for composeEmoji in the compose window's tests, whose TipTap editor is a
// fake with no ProseMirror state for the send-time emoji pass to read. The pass itself is tested against
// a real editor in composeEmoji.test.ts. A test file opts in with:
//
//   vi.mock('../composeEmoji', () => import('../test/composeEmojiStub'))
export const EmojiInput = {}

export function translateTrailingEmoji(): void {}
