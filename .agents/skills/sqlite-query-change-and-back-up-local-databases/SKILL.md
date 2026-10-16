---
name: sqlite-query-change-and-back-up-local-databases
description: Query, change, and back up a local SQLite database with the sqlite3 CLI. Use when the data already lives in a .db file.
---

# SQLite — Query, Change, and Back Up Local Databases

Work on a local SQLite database file. The `sqlite3` CLI is enough for inspection, one-off queries, and backups. Use Python's stdlib `sqlite3` when a value comes from outside the command you typed.

## When to use

The user has a `.db` file, or they want a small single-file database for notes, logs, or agent state. This skill covers that file. It does not cover a client/server database.

## Install

`sqlite3` is often already on the machine. Check, then install the package only if the command is missing.

```sh
sqlite3 -version
```

```sh
# Debian / Ubuntu
sudo apt-get install sqlite3

# macOS already ships sqlite3. Homebrew installs a newer build:
brew install sqlite
```

Python needs no extra package. `import sqlite3` is in the standard library.

## Commands

Create a database, add a table, and read it back. `-json` prints rows an agent can parse.

```sh
sqlite3 notes.db "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL);"
sqlite3 notes.db "INSERT INTO notes (body) VALUES ('hello');"
sqlite3 -json notes.db "SELECT id, body FROM notes;"
```

See what is in the file:

```sh
sqlite3 notes.db ".tables"
sqlite3 notes.db ".schema"
```

Turn foreign keys on for a session, then check them. SQLite leaves them off until you ask.

```sh
sqlite3 notes.db "PRAGMA foreign_keys = ON; PRAGMA foreign_keys;"
```

Back up a live database with the CLI backup command, then confirm the copy. Copying the file with `cp` can catch a WAL database mid-write.

```sh
sqlite3 notes.db ".backup 'notes-backup.db'"
sqlite3 notes-backup.db "PRAGMA integrity_check;"
```

When a value is not a literal you typed, bind it from Python:

```python
import sqlite3

con = sqlite3.connect("notes.db")
con.execute("PRAGMA foreign_keys = ON")
row = con.execute("SELECT body FROM notes WHERE id = ?", (1,)).fetchone()
print(row[0])
```

Run an untrusted SQL script with the safe mode so it cannot shell out or load extensions:

```sh
sqlite3 --safe notes.db
```

## Gotchas

- `PRAGMA foreign_keys` defaults to `0`. Set it on every connection that should enforce foreign keys.
- A WAL database keeps `dbname-wal` and `dbname-shm` beside the main file. Use `.backup` so those pages are included.
- `INTEGER PRIMARY KEY` is the rowid. Inserting without an id assigns one.
- Build queries with `?` placeholders. Concatenating text into SQL is how a value becomes a statement.
- `integrity_check` prints `ok` when the copy is consistent.
