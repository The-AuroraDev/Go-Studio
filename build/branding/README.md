# 品牌资源

Go Studio 的标识与图标资产。**唯一真源是 `gostudio-mark.svg`**，其余全部是生成产物。

## 文件职责

| 文件 | 角色 | 说明 |
|---|---|---|
| `gostudio-mark.svg` | **主文件，人工维护** | 自绘的吉祥物 + 触控笔，viewBox `0 0 280 290` |
| `../linux/go-studio.svg` | 生成 | 正方形裁剪版，供 hicolor/scalable 使用 |
| `../appicon.png` | 生成 | 1024×1024，Wails 用它生成 macOS icns |
| `../windows/icon.ico` | 生成 | 16/24/32/48/64/128/256 多尺寸 |
| `../linux/icons/*.png` | 生成 | 常见尺寸位图，兼容不认可缩放图标的桌面 |
| `../../assets/icons/go-brand.svg` | 生成 | 欢迎页品牌图标，正方形 256×256 |

## 重新生成

```sh
python3 -m venv .venv
.venv/bin/pip install cairosvg
.venv/bin/python scripts/render-icons.py
```

SVG 衍生物只需要标准库；位图需要 `cairosvg`（内含 Pillow）。缺少 `cairosvg` 时脚本会跳过位图并返回非零码，不会假装已经更新。

`make check` 里的 `check-icons.sh` 会校验衍生物是否齐备、位图边长是否与文件名一致，因此改了主文件忘记重新生成会被拦下。

## 调整正方形裁剪区

主文件的 viewBox 与内容外接框目前推出 `SQUARE_VIEWBOX = "14.5 16 264 264"`。
改动主文件的 viewBox 会触发 `render-icons.py` 的断言失败，防止静默生成错误的裁剪结果。届时按下面的步骤重算：

1. 渲染一版带背景的 PNG，肉眼确认内容外接框
2. 取 `边长 = max(内容宽, 内容高)`，再按内容中心居中写出 `viewBox="x y 边长 边长"`
3. 更新 `render-icons.py` 里的 `EXPECTED_MASTER_VIEWBOX` 与 `SQUARE_VIEWBOX`

## 设计约束

依据 `AIdocs/DESIGN.md` 第 9、12 节：

- 矢量、单色描边，无位图、无渐变、无 emoji
- 描边默认 `#000000` 5 单位；**小部件必须单独收窄描边**，否则会被黑色描边涂满
  （笔尖三角只有 6 单位宽，沿用 5 单位描边时黄色笔尖会完全消失）
- 描边与填充分离：纯色细节加 `stroke="none"`
- 应用图标允许彩色；界面内的 Go 语言标记另见 `assets/icons/go.svg`

## 商标

本标识为自绘，**不是 Go 官方标识**，不含 Go 项目的商标素材，因此不涉及 Go 商标授权。

`assets/icons/go.svg`（单色）仍派生自 Go 官方 gopher，用于在文件树里标记 `.go`、`go.mod` 等文件，属于指称性使用。若要彻底规避，可把它也换成本标识的 monochrome 版本。
