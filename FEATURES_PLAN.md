# PigeonPost: Features Plan

> The triaged feature backlog: net-new candidates beyond what has shipped and beyond
> the committed roadmap (the README's Planned line). Verdict taxonomy
> (**Do** / **Design** / **Defer** / **Park**), read against the domain / application
> / adapter split, the coverage gate and the RFC-mirrored conformance tests. Ordering
> protects the paid-down debt: expose built value and confirm the trust boundary
> first, hold the platform-scale items until their prerequisites exist so nothing
> lands as new debt.

---

## Deferred / Park (recorded so the gaps are known, not accidental)

**Encrypted mail store at rest** (Parked, decided 2026-07-14)
Encrypting the local cache on disk (SQLCipher or equivalent) is parked to keep the
pure-Go single-binary build: SQLCipher and every mature transparent-SQLite-encryption
option is a C library requiring CGO, which would put a C toolchain in every build and
break easy cross-compilation right before the macOS/Linux release work. There is no
production-grade pure-Go equivalent today. The GH Pages site states the honest scope
(passwords in the OS keychain; the local cache is not encrypted at rest; pair with OS
disk encryption such as BitLocker or FileVault). Revisit if a trustworthy pure-Go
encrypted driver matures. If ever done, whole-DB encryption is the only coherent
shape: the search index lives in the same database file and stores its own copy of
indexed body text, so column-level encryption would leave the index as a plaintext
copy.

**PGP / S-MIME end-to-end encryption** (Park; not a current goal)
Real minefield (key management, Autocrypt, "is it actually encrypted" UX) and not
the primary goal. Not in core. Candidate future home: a plugin (see below). Note:
this is distinct from **Encrypted store at rest** above (at-rest protection of the
local cache, parked for its own reason).

**Plugin architecture** (Park / Design-later; strategic platform item)
The eventual home for PGP and other opt-in extensions. Recorded honestly as its own
significant commitment, **not** a casual "later": a plugin system is a platform
decision and crypto-in-a-plugin (a third party touching private keys/decryption)
carries its own trust surface. If pursued, it gets its own design plan defining the
extension API, sandboxing and trust model before any feature rides on it.

**Local-archive import** (Park; the gap behind "add account + sync")
Importing mail that does **not** live on a server: POP3 locals, Thunderbird "Local
Folders", mbox / .pst exports, server-deleted-but-kept mail. Out of scope now;
revisit only if the audience shows demand. Keep site/onboarding copy scoped to
server-side mail so this gap is never implied away.

**Export / mbox backup** (Park; server is the backup)
When mail lives server-side, provable local export is a weak story and users can
save individual messages or re-sync accounts as needed. Revisit only if meaningful
local-only archiving lands (would pair with local-archive import).

---

## Won't-do (protect the thesis; confirmed)

- **No** ambient AI summarisation of mail by default; **no** ML "priority inbox"
  reordering. Anything AI is opt-in, on-demand and clearly disclosed, never
  silently interpreting or reordering the user's mail. "Calm and predictable" is the
  promise; ambient intelligence breaks it.
- **No** social / feed / "important people are posting" surfaces.
- **No** engagement mechanics or nudges.
- **No** hosted PigeonPost account or relay, including as a fix for the
  snooze/send-later "app must be running" constraint. The absence of a cloud in
  between is a load-bearing wall, not a limitation to engineer away.

---

## Suggested sequence

1. Everything under Park stays parked until its prerequisite/demand appears.
