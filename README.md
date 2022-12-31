# HDL插件

HDL插件主要功能是提供HTTP-FLV协议的访问

## 插件地址

https://github.com/Monibuca/plugin-hdl

## 插件引入
```go
import (
    _ "m7s.live/plugin/hdl/v4"
)
```

## 默认插件配置

```yaml
hdl
    http:
        listenaddr: :8080
        listenaddrtls: ""
        certfile: ""
        keyfile: ""
        cors: true
        username: ""
        password: ""
    publish:
        pubaudio: true
        pubvideo: true
        kickexist: false
        publishtimeout: 10
        waitclosetimeout: 0
        delayclosetimeout: 0
    subscribe:
        subaudio: true
        subvideo: true
        iframeonly: false
        waittimeout: 10
    pull:
        repull: 0
        pullonstart: {}
        pullonsub: {}
```
## 插件功能

### 从m7s拉取http-flv协议流
如果m7s中已经存在live/test流的话就可以用http-flv协议进行播放
如果监听端口不配置则公用全局的HTTP端口(默认8080)
```bash
ffplay http://localhost:8080/hdl/live/test.flv
```
### m7s从远程拉取http-flv协议流

可调用接口
`/hdl/api/pull?target=[HTTP-FLV地址]&streamPath=[流标识]&save=[是否保存配置（留空则不保存）]`

### 获取所有HDL流列表
`/hdl/api/list`