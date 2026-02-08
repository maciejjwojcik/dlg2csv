# dlg2csv

`dlg2csv` is a command-line tool for exporting **Infinity Engine dialogue sources**
(WeiDU `.d` + `.tra`) into **translator-friendly CSV files**.

It is designed to support the common community workflow where:
- dialogue structure lives in version-controlled mod repositories (often on GitHub),
- translation work is done collaboratively in spreadsheets (e.g. Google Sheets),
- and final `.tra` files are generated from those spreadsheets.

The tool focuses on **readability, context, and stability**, rather than raw binary formats.

## What it does (planned / work in progress)

- Parse WeiDU dialogue sources (`.d`) and translation files (`.tra`)
- Export dialogues into CSV with:
  - NPC lines and player responses shown together
  - dialogue flow references (next states)
  - enough context for translators to work comfortably
- Export all non-dialogue strings from `.tra` files as well
- Generate clean, canonical `.tra` output from translated CSV files

## What it does NOT do

- It does not include any game assets or copyrighted content
- It does not modify binary `.dlg` files
- It does not require translators to use Git or GitHub

## Status

Early development / MVP stage.  
The initial goal is a stable export → translate → rebuild `.tra` workflow.

## Author

Maciej Wójcik/cherrycoke2l

## License

MIT