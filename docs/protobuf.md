# Protobuf

## VS Code

```.json
{
    "protoc": {
        "options": [
            "--proto_path={workspaceFolder}/proto",
        ]
    }
}
```

## Generate
```sh
protoc -I proto/ proto/*.proto --go_out=paths=source_relative:generated/go --experimental_allow_proto3_optional
```