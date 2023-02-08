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
hdl:
    http:
      listenaddr: :8080 # 网关地址，用于访问API
      listenaddrtls: ""  # 用于HTTPS方式访问API的端口配置
      certfile: ""
      keyfile: ""
      cors: true  # 是否自动添加cors头
      username: ""  # 用户名和密码，用于API访问时的基本身份认证
      password: ""
    publish:
        pubaudio: true # 是否发布音频流
        pubvideo: true # 是否发布视频流
        kickexist: false # 剔出已经存在的发布者，用于顶替原有发布者
        publishtimeout: 10s # 发布流默认过期时间，超过该时间发布者没有恢复流将被删除
        delayclosetimeout: 0 # 自动关闭触发后延迟的时间(期间内如果有新的订阅则取消触发关闭)，0为关闭该功能，保持连接。
        waitclosetimeout: 0 # 发布者断开后等待时间，超过该时间发布者没有恢复流将被删除，0为关闭该功能，由订阅者决定是否删除
        buffertime: 0 # 缓存时间，用于时光回溯，0为关闭缓存
    subscribe:
        subaudio: true # 是否订阅音频流
        subvideo: true # 是否订阅视频流
        subaudioargname: ats # 订阅音频轨道参数名
        subvideoargname: vts # 订阅视频轨道参数名
        subdataargname: dts # 订阅数据轨道参数名
        subaudiotracks: [] # 订阅音频轨道名称列表
        subvideotracks: [] # 订阅视频轨道名称列表
        submode: 0 # 订阅模式，0为跳帧追赶模式，1为不追赶（多用于录制），2为时光回溯模式
        iframeonly: false # 只订阅关键帧
        waittimeout: 10s # 等待发布者的超时时间，用于订阅尚未发布的流
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