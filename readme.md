# dlg2csv

`dlg2csv` is a command-line tool for exporting **Infinity Engine dialogue sources**
(WeiDU `.d` + `.tra`) into **translator-friendly CSV files**.

It is designed to support the common community workflow where:
- dialogue structure lives in version-controlled mod repositories (often on GitHub),
- translation work is done collaboratively in spreadsheets (e.g. Google Sheets),
- and final `.tra` files are generated from those spreadsheets.

The tool focuses on **readability, context, and stability**, rather than raw binary formats.

## What it does (planned / work in progress)

- Parse WeiDU dialogue sources (`.d`) and translation files (`.tra`),
- Export dialogues into CSV with:
  - NPC lines and player responses shown together,
  - dialogue flow references (next states),
  - enough context for translators to work comfortably.
- Export all non-dialogue strings from `.tra` files as well,
- Generate clean, canonical `.tra` output from translated CSV files (to do).

## What it does NOT do

- It does not include any game assets or copyrighted content,
- It does not modify binary `.dlg` files,
- It does not require translators to use Git or GitHub.

## Usage

### Basic usage

Run `dlg2csv` in a directory containing WeiDU `.d` and `.tra` files:

```bash
dlg2csv
```

This will:

- read `.tra` and `.d` files from the current directory,
- generate CSV files with dialogue content and strings.

### Separate directories for `.tra` and `.d``

```bash
dlg2csv <traDir> <dDir>
```

Example:
```bash
dlg2csv language/english dlg/dialogues_compile
```

In this case:

- `.tra` files are read from `language/english`,

- `.d` files are read from `dlg/dialogues_compile`.

### Output

The tool generates one CSV per `.tra` source file. The CSV files are intended to be opened and edited in spreadsheet tools
such as Google Sheets or Excel.

Columns for translated text (male/female variants) are intentionally left empty
and meant to be filled by translators.

## Status

Early development / MVP stage.  
The initial goal is a stable export → translate → rebuild `.tra` workflow.

## Author

Maciej Wójcik/cherrycoke2l

## License

MIT