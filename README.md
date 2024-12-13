# Usage


## Run da-server
```bash
$ cat config.json

{
	"SequencerIP": "127.0.0.1",
	"ListenAddr": "0.0.0.0:8888",
	"StorePath":  "/root/da/data"
}

$ go run main.go da start --config config.json
```


## Get blob with da-client

```bash
go run main.go da download --rpc DA_SERVER_URL --blob_hash BLOB_HASH
```

(Replace `DA_SERVER_URL` and `BLOB_HASH` with proper values.)