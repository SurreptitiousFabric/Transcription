# Census Transcription TUI

This project provides a terminal user interface for manually entering UK census data. The information you type can be saved as HTML for viewing in any browser.

## Running

Ensure a Go toolchain is available then start the program with:

```
go run main.go
```

On start you are shown a menu of census years from 1841 through 1921. Use the
up and down arrows to highlight a year and press **Enter** to continue.

## Key bindings

- **Ctrl-H** – edit the header
- **Ctrl-B** – edit the body rows
- **Ctrl-F** – edit the footer
- **Tab** / **Shift-Tab** – move between fields
- **↑** / **↓** – navigate rows in body mode
- **Ctrl-N** – clear the current body row
- **Ctrl-O** – open a previously saved HTML file
- **Ctrl-W** – save the form as `census.html`
- **Esc** (or **Ctrl-C**) – quit the program

The currently active mode and a reminder of these keys are displayed in the
title bar while you work.

## Example output

A snippet of the generated HTML looks like:

```
  <colgroup><col style="width:8.33%" span="7"></colgroup>
  <thead>
    <tr><th colspan="7" align="center">
      The undermentioned Houses are situate within the Boundaries of the
    </th></tr>
```

Rows of data are written using `<PersonRef>` and `<Mark>` elements, for example:

```
    <tr><td></td><td></td><td></td><td></td><td><PersonRef detlnk="dpR3C5">George M do</PersonRef></td><td><Mark ref="R3C6">Servant</Mark></td><td></td><td><Mark ref="R3C8">22</Mark></td><td></td><td><Mark ref="R3C10">Clergyman</Mark></td><td><PlaceRef detlnk="dwR3C11">Essex, Great Canfield</PlaceRef></td><td></td></tr>
```

Any census page saved this way can be reopened with **Ctrl-O** or simply viewed
in a web browser.
