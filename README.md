# GoRenameMedia
A tool to rename media based on data retrieved via exiftool

## Export File Tags

This is a small utility to rename media file's with the date/time it was taken. 

Makes use of the [exiftool](https://exiftool.org) executable v12.65 (saved in this repo, and already outdated)

Run the executable with a filepath
```
.\gorenamemedia_v0.0.1.exe <file-path-to-root-dir>
```

### Development

Run the project with
```
go run . <file-path>
```

#### If you run this on your target machine, it should default to the correct $GOOS and $GOARCH to build:
```
go build -o gorenamemedia_v0.0.1.exe main.go
```
