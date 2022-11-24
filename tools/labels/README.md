# Label Syncing

This folder contains the sources of the `snyclabels` tool that can be used to
make the labels of the repositories of the Carbyne Stack GitHub organization
adhere to a scheme defined in a configuration file.

## Installing

You can easily install the tool from source using

```shell
git clone git@github.com:carbynestack/.github.git
cd .github/tools/labels
go install
```

## Configuration

A configuration file called `config.json` has to be made available in the
working directory. The structure of the configuration file is as follows:

```json lines
{
  // The repositories to be processed
  "repositories": [
    "repo-1",
    ...,
    "repo-n"
  ],
  "labels": {
    // If set to `true` labels that are not listed in the `desired` array will
    // be deleted. 
    "delete-obsolete": true,
    // The labels to be created / updated
    "desired": [
      {
        "name": "...",
        "description": "...",
        "color": "...",
        "replaces": "..." // If given, the given existing label will be updated
      }
    ]
  }
}
```

The `config.json` available in `.github/tools/labels` defines the current scheme
used for the Carbyne Stack repositories.

## Usage

You have to provide an access token with `repo` scope via the environment
variable `GITHUB_TOKEN`.

The tool can be invoked as follows

```shell
GITHUB_TOKEN=<redacted> synclabels
```
