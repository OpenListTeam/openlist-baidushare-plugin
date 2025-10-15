# OpenList Baidu Share Plugin (测试版)

**请注意：** 当前版本为测试版，可能存在一些未知问题。

## 构建命令

您可以使用以下命令来构建插件的 `.wasm` 文件：

```bash
tinygo build -target=wasip1 -no-debug -buildmode=c-shared -o plugin.wasm .
```