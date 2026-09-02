# Relic

**Your photo library, smaller and safer—right in your browser.**  
Relic is a self‑hosted web app that shrinks your photo archive without ever losing a single pixel. Upload your shoots, let the backend find hidden space savings, and restore any file bit‑for‑bit identical to the original—all through a clean web interface.

---

## What it does (in plain English)

- **Makes your library take less disk space** – by understanding what’s inside each file (RAWs, embedded previews, duplicates) and compressing smarter.
- **Guarantees every byte is intact** – every file gets a digital fingerprint on upload, and every restore is verified against that fingerprint. If something doesn’t match, you get an error—not a corrupted photo.
- **Removes duplicate files automatically** – identical photos are stored once and referenced many times, freeing up space.
- **Gives you a timeline and search** – browse your archived shoots by date, sequence, or file name.

> **Status: early / experimental.** Numbers below are targets—real results will be published once the benchmark suite is ready.

---

## The honest truth about compression

You can’t magically shrink an already‑compressed JPEG to 12% of its size—math doesn’t allow it. Anyone promising that is selling something.

But “already compressed” doesn’t mean “as small as it could possibly be.” That’s where Relic looks for savings:

| File type | Where the savings come from | Typical target |
|-----------|-----------------------------|----------------|
| JPEG | Re‑coding the data (JPEG’s old Huffman coding left some space on the table) | ~20% |
| Uncompressed RAW | Smart prediction + modern compression on the raw sensor data | ~30–50% |
| Compressed RAW | Extracting the full‑resolution JPEG preview that’s already inside, then transcoding it | ~5–15% |
| TIFF / PNG | Depends on the source, but often significant | ~20–40% |
| Modern video (H.264/HEVC/AV1) | Nothing—those codecs are already extremely efficient. Relic skips them. | ~0% |
| Exact duplicates | Stored only once, referenced many times | Free (deduplication) |

Overall savings depend on how many duplicates you have, so Relic reports **compression ratio** (actual shrinking) and **dedup ratio** (space saved by removing duplicates) separately.

---

## How Relic works (behind the scenes)

1. **Upload** – You drag‑and‑drop a folder (or select individual files) through the web interface.
2. **Fingerprint** – Every file is hashed with a fast algorithm (BLAKE3) to give it a unique ID.
3. **Deduplicate** – Identical files are stored only once; duplicates just point to the same chunk.
4. **Route by format** – JPEGs, RAWs, TIFFs, etc. each get their own optimised compression; modern video is skipped.
5. **Test before saving** – We compress in memory, then decompress and compare to the original. If it doesn’t match exactly, we fall back to a generic compressor—so we never write anything that can’t be perfectly restored.
6. **Store with an index** – A SQLite database tracks where every piece of every file lives. Restoring one photo reads only that photo’s chunks—no need to unpack the whole archive.

---

## Integrity – how we keep your photos safe

- Every uploaded file is hashed.
- When you restore, we re‑hash the restored file and compare.
- **If they don’t match, we refuse to give you the file.** No warnings—just a clear error.

**Two important notes:**

- **Deduplication concentrates risk.** One corrupted chunk can break every file that references it. We’re adding error‑correcting parity (like RAID) to protect against this—but it’s not ready yet. Treat Relic as one copy, not a backup.
- **Relic is not a backup.** It’s still just one copy in one place. Keep three copies on two different media, with one offsite—that’s the golden rule.

---

## Getting started (self‑hosted)

1. Clone the repository.
2. Build the backend (Go) and frontend (React/TypeScript).
3. Run the server – it serves the web UI and exposes an API.
4. Open your browser, upload your first shoot, and watch the space savings appear.

Detailed setup instructions are in the repo’s `docs/` folder.

---

## What Relic is **not**

- A replacement for Lightroom or a photo editor.
- A video transcoder or cloud storage provider.
- A backup tool by itself.

It’s an archival storage engine—with a photographer‑friendly web interface.

---

## Built with

- **Go** – for the backend (fast, reliable, easy to deploy).
- **React + TypeScript** – for the frontend (clean, modern UI).
- **Pure‑Go SQLite** – no external database dependencies.
- **Modern compression libraries** – for speed and efficiency.

---

## The big question

> How much smaller can a real photography library get while guaranteeing every original file reconstructs exactly?

We don’t make claims until they’ve been measured. Every number in this document is a target—real results will be published once our benchmark suite is complete.

---

Built for [Hack Club Third Space](https://thirdspace.hackclub.com/).