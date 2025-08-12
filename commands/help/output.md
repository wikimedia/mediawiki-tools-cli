# How to use --output, --filter and --format

Many commands that print data back to you will come with these three flags by default.
They allow you to change how the data is returned to you.

There are currently 5 available output types.

 - `table`: A pretty table
 - `json`: JSON
 - `template`: Golang templates (gotmpl)
 - `ack`: Ack style
 - `web`: Open in a web browser

These are most easily testable using the `mw version` command, which supports all output types, filters and format options.

Other commands will provide more complex output, filtering and formatting options, but the same principles apply.

## Table

`mw version --output=table` outputs a 2 column table.

```
Version Information  Value
BuildDate            2022-10-07T15:06:15Z
Version              latest
```

## Ack

`mw version --output=ack` outputs a single ack section with 2 rows of information.

```
Version Information:
BuildDate: 2022-10-07T15:06:15Z
Version: latest
```

## JSON

`mw version --output=json` outputs a single json object with 2 keys of information.


```json
{
  "BuildDate": "2022-10-07T15:08:29Z",
  "Version" :"latest"
}
```

This can be manipulated via `jq` compatible syntax to the `--format` flag.

For example, `mwdev version --output=json --format=.Version`

```
"latest"
```

## Template

Template needs a format right away in order to output.

`mw version --output=template --format={{.}}` produces the following data.

```
map[BuildDate:2022-10-07T15:08:29Z Version:latest]
```

This can be manipulated through gotmpl syntax, for example `mw version --output=template --format={{.Version}}`

```
latest
```

## Web

`mw version --output=web` will open the output in a web browser.