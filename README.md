## 缘起
由于一直习惯使用clash核心来进行翻墙，但是老是有一个黑漆漆的cmd窗口在任务栏上，让人看来非常不爽，所以这里简单使用go来实现了一个托盘程序，用来直接控制clash核心，这个只会在托盘中显示，而不会在任务栏中显示，让人看起来舒服一点。
> 习惯使用配置文件，一般的clash gui感觉太重了，觉得不好用

## 编译
### 1. 生成资源文件
可以自定义exe文件的图标，需要icon类型，名字命名成clash.ico，放在main.go同目录下
```batch
rsrc -manifest app.manifest -o app.syso -ico clash.ico
```

### 2. 编译exe文件
```batch
go build -o build/ClashTray.exe -trimpath -ldflags "-H windowsgui -s -w"
```

## 使用
### 1. 目录存放
将编译后的exe文件放到mihomo.exe文件同目录下，目录结构如下
```
|--ClashTray.exe
|--mihomo.exe
|--config
   |--config.yaml
```
> mihomo需要的其他数据文件也都是在config目录下

### 2. 启动
直接双击ClashTray.exe启动程序
> 程序会请求管理员权限，用于tun接口的创建

可以看到系统托盘中已经有一个Clash图标了，点击该图标，选择`Start Clash`，来启动Clash

### 3. 关闭
- 之后可以通过点击`Stop Clash`，来关闭Clash程序
- 要退出程序直接点击`Quit`，同时也会自动关闭Clash程序

> 在exe文件同目录下会生成clash.log，用于记录clash输出的日志