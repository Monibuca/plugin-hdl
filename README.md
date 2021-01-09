# hdlplugin
http-flv for monibuca

实现了HTTP-FLV协议，适合对接CDN厂商，作为拉流协议。

## 插件名称

HDL

## 配置
```toml
[HDL]
ListenAddr = ":2020"
ListenAddrTLS = ":2021"
CertFile = "file.cert"
KeyFile = "file.key"
```

用于监听http和https端口

- 如果不设置ListenAddr和ListenAddrTLS，将公用网关的端口监听