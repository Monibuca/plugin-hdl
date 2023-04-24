_[简体中文](https://github.com/Monibuca/plugin-hdl) | English_
# HDL Plugin

The main function of the HDL plugin is to provide access to the HTTP-FLV protocol.

## Plugin Address

https://github.com/Monibuca/plugin-hdl

## Plugin Introduction
```go
import (
    _ "m7s.live/plugin/hdl/v4"
)
```

## Default Plugin Configuration

```yaml
hdl:
  http: # Refer to global configuration for format
  publish: # Refer to global configuration for format
  subscribe: # Refer to global configuration for format
  pull: # Format: https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
```

## Plugin Features

### Pulling HTTP-FLV Streams from M7S

If the live/test stream already exists in M7S, then HTTP-FLV protocol can be used for playback. If the listening port is not configured, then the global HTTP port is used (default 8080).

```bash
ffplay http://localhost:8080/hdl/live/test.flv
```

### M7S Pull HTTP-FLV Streams from Remote

The available API is:
`/hdl/api/pull?target=[HTTP-FLV address]&streamPath=[stream identifier]&save=[whether to save configuration (leave blank if not)]`

### Get a List of All HDL Streams

`/hdl/api/list`