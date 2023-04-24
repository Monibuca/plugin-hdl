_[English](https://github.com/Monibuca/plugin-hdl/blob/v4/README.en.md) | 简体中文_
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
  http: # 格式参考全局配置
  publish: # 格式参考全局配置
  subscribe: # 格式参考全局配置
  pull: # 格式 https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
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