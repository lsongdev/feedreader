# feedreader

> FeedReader - Self-hosted RSS Feed Aggregator Service

## Installation

```shell
curl -L https://github.com/song940/feedreader/releases/download/latest/reader-linux-amd64 -o /usr/bin/feedreader
chmod +x /usr/bin/feedreader
```

Create a service file in `/etc/systemd/system/feedreader.service`:

```ini
[Unit]
Description=FeedReader - Self-hosted RSS Feed Aggregator Service
Documentation=https://github.com/song940/feedreader
After=network-online.target
Wants=network-online.target systemd-networkd-wait-online.service

[Service]
ExecStart=/usr/local/bin/feedreader

[Install]
WantedBy=multi-user.target
```

## Usage

```shell
Usage of feedreader:
  -d string
        working directory (default "/etc/feedreader")
  -l string
        address to listen (default ":8080")
```

## Contributors

## License

MIT