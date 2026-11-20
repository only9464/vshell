# 原版 4.9.3（Linux 构建）业务控制器方法地址

由 `scripts/go-symbols.py` 从 `.gopclntab` 恢复：garble 改了 pclntab 头部魔数与包名，
但**控制器类型与方法名保留**，因此可以按名字直接定位。

> 地址为文件内虚拟地址，可直接用于 `objdump -d --start-address=<addr>` 或 IDA/Ghidra 跳转。
> Windows 版（`.re` 里的 FUN_ 地址）与本表**不同**，跨版本只按名字/字符串对应。

## ClientController

| 方法 | 地址 | 字节数 |
|---|---|---|
| List | `0000000001820e00` | 6912 |
| Add | `0000000001822900` | 896 |
| EditRemark | `0000000001822c80` | 256 |
| DelFile | `0000000001822d80` | 448 |
| DelProcess | `0000000001822f40` | 320 |
| DelList | `0000000001823080` | ? |
| JsonErr | `0000000001865b60` | 128 |
| JsonGet | `0000000001865be0` | 416 |
| JsonGetBool | `0000000001865d80` | 128 |
| JsonGetInt | `0000000001865e00` | 128 |
| JsonGetIntList | `0000000001865e80` | 128 |
| JsonGetStr | `0000000001865f00` | 128 |
| JsonGetStrList | `0000000001865f80` | 128 |
| JsonOkMessage | `0000000001866000` | 128 |
| JsonOkResult | `0000000001866080` | 128 |
| JsonTimeout | `0000000001866100` | 736 |
| Prepare | `00000000018663e0` | 1664 |
| SessionRegenerateID | `0000000001866a60` | 992 |
| URLFor | `0000000001866e40` | 192 |
| XSRFFormHTML | `0000000001866f00` | ? |

## DashboardController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Info | `00000000018232e0` | ? |
| JsonErr | `00000000018681e0` | 128 |
| JsonGet | `0000000001868260` | 416 |
| JsonGetBool | `0000000001868400` | 128 |
| JsonGetInt | `0000000001868480` | 128 |
| JsonGetIntList | `0000000001868500` | 128 |
| JsonGetStr | `0000000001868580` | 128 |
| JsonGetStrList | `0000000001868600` | 128 |
| JsonOkMessage | `0000000001868680` | 128 |
| JsonOkResult | `0000000001868700` | 128 |
| JsonTimeout | `0000000001868780` | 736 |
| Prepare | `0000000001868a60` | 1664 |
| SessionRegenerateID | `00000000018690e0` | 992 |
| URLFor | `00000000018694c0` | 192 |
| XSRFFormHTML | `0000000001869580` | ? |

## DownloadController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Stage | `0000000001823960` | 3552 |
| Stageless | `0000000001824740` | 4704 |
| Listen | `00000000018259a0` | 4576 |
| Shellcode | `0000000001826b80` | 10368 |
| Dll | `0000000001829400` | 4544 |
| ListenDll | `000000000182a5c0` | ? |
| JsonErr | `000000000186a860` | 128 |
| JsonGet | `000000000186a8e0` | 416 |
| JsonGetBool | `000000000186aa80` | 128 |
| JsonGetInt | `000000000186ab00` | 128 |
| JsonGetIntList | `000000000186ab80` | 128 |
| JsonGetStr | `000000000186ac00` | 128 |
| JsonGetStrList | `000000000186ac80` | 128 |
| JsonOkMessage | `000000000186ad00` | 128 |
| JsonOkResult | `000000000186ad80` | 128 |
| JsonTimeout | `000000000186ae00` | 736 |
| Prepare | `000000000186b0e0` | 1664 |
| SessionRegenerateID | `000000000186b760` | 992 |
| URLFor | `000000000186bb40` | 192 |
| XSRFFormHTML | `000000000186bc00` | ? |

## FileController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Getdisk | `000000000182b6e0` | 1440 |
| Ls | `000000000182bc80` | 3776 |
| Mv | `000000000182cb40` | 1248 |
| Rm | `000000000182d020` | 864 |
| RmList | `000000000182d380` | 736 |
| Cat | `000000000182d660` | 1088 |
| Touch | `000000000182daa0` | 768 |
| Modifytime | `000000000182dda0` | 1344 |
| Mkdir | `000000000182e2e0` | 1024 |
| Edit | `000000000182e6e0` | 832 |
| Wget | `000000000182ea20` | 992 |
| Upload | `000000000182ee00` | 1536 |
| DownloadToServer | `000000000182f400` | 3232 |
| GetDownloadPer | `00000000018300a0` | 992 |
| DownloadToBrowser | `0000000001830480` | 576 |
| DownloadToOSS | `00000000018306c0` | ? |
| JsonErr | `000000000186cee0` | 128 |
| JsonGet | `000000000186cf60` | 416 |
| JsonGetBool | `000000000186d100` | 128 |
| JsonGetInt | `000000000186d180` | 128 |
| JsonGetIntList | `000000000186d200` | 128 |
| JsonGetStr | `000000000186d280` | 128 |
| JsonGetStrList | `000000000186d300` | 128 |
| JsonOkMessage | `000000000186d380` | 128 |
| JsonOkResult | `000000000186d400` | 128 |
| JsonTimeout | `000000000186d480` | 736 |
| Prepare | `000000000186d760` | 1664 |
| SessionRegenerateID | `000000000186dde0` | 992 |
| URLFor | `000000000186e1c0` | 192 |
| XSRFFormHTML | `000000000186e280` | ? |

## InstallController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Install | `0000000001830ae0` | 672 |
| Remove | `0000000001830d80` | ? |
| JsonErr | `000000000186f560` | 128 |
| JsonGet | `000000000186f5e0` | 416 |
| JsonGetBool | `000000000186f780` | 128 |
| JsonGetInt | `000000000186f800` | 128 |
| JsonGetIntList | `000000000186f880` | 128 |
| JsonGetStr | `000000000186f900` | 128 |
| JsonGetStrList | `000000000186f980` | 128 |
| JsonOkMessage | `000000000186fa00` | 128 |
| JsonOkResult | `000000000186fa80` | 128 |
| JsonTimeout | `000000000186fb00` | 736 |
| Prepare | `000000000186fde0` | 1664 |
| SessionRegenerateID | `0000000001870460` | 992 |
| URLFor | `0000000001870840` | 192 |
| XSRFFormHTML | `0000000001870900` | ? |

## ListenerController

| 方法 | 地址 | 字节数 |
|---|---|---|
| List | `00000000018310a0` | 7360 |
| EditRemark | `0000000001832d60` | 352 |
| DelList | `0000000001832ec0` | 320 |
| Del | `0000000001833000` | 256 |
| Stop | `0000000001833100` | 256 |
| Start | `0000000001833200` | 256 |
| Edit | `0000000001833300` | 1568 |
| Add | `0000000001833920` | ? |
| JsonErr | `0000000001871be0` | 128 |
| JsonGet | `0000000001871c60` | 416 |
| JsonGetBool | `0000000001871e00` | 128 |
| JsonGetInt | `0000000001871e80` | 128 |
| JsonGetIntList | `0000000001871f00` | 128 |
| JsonGetStr | `0000000001871f80` | 128 |
| JsonGetStrList | `0000000001872000` | 128 |
| JsonOkMessage | `0000000001872080` | 128 |
| JsonOkResult | `0000000001872100` | 128 |
| JsonTimeout | `0000000001872180` | 736 |
| Prepare | `0000000001872460` | 1664 |
| SessionRegenerateID | `0000000001872ae0` | 992 |
| URLFor | `0000000001872ec0` | 192 |
| XSRFFormHTML | `0000000001872f80` | ? |

## LoginController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Login | `0000000001834020` | 1408 |
| Logout | `00000000018345a0` | 96 |
| GetUserInfo | `0000000001834600` | ? |
| JsonErr | `0000000001874260` | 128 |
| JsonGet | `00000000018742e0` | 416 |
| JsonGetBool | `0000000001874480` | 128 |
| JsonGetInt | `0000000001874500` | 128 |
| JsonGetIntList | `0000000001874580` | 128 |
| JsonGetStr | `0000000001874600` | 128 |
| JsonGetStrList | `0000000001874680` | 128 |
| JsonOkMessage | `0000000001874700` | 128 |
| JsonOkResult | `0000000001874780` | 128 |
| JsonTimeout | `0000000001874800` | 736 |
| Prepare | `0000000001874ae0` | 1664 |
| SessionRegenerateID | `0000000001875160` | 992 |
| URLFor | `0000000001875540` | 192 |
| XSRFFormHTML | `0000000001875600` | ? |

## RunnerController

| 方法 | 地址 | 字节数 |
|---|---|---|
| List | `00000000018349e0` | 1152 |
| RunPlugin | `0000000001834e60` | 3456 |
| Upload | `0000000001835be0` | ? |
| JsonErr | `00000000018768e0` | 128 |
| JsonGet | `0000000001876960` | 416 |
| JsonGetBool | `0000000001876b00` | 128 |
| JsonGetInt | `0000000001876b80` | 128 |
| JsonGetIntList | `0000000001876c00` | 128 |
| JsonGetStr | `0000000001876c80` | 128 |
| JsonGetStrList | `0000000001876d00` | 128 |
| JsonOkMessage | `0000000001876d80` | 128 |
| JsonOkResult | `0000000001876e00` | 128 |
| JsonTimeout | `0000000001876e80` | 736 |
| Prepare | `0000000001877160` | 1664 |
| SessionRegenerateID | `00000000018777e0` | 992 |
| URLFor | `0000000001877bc0` | 192 |
| XSRFFormHTML | `0000000001877c80` | ? |

## ScreenController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Ws | `0000000001835f20` | ? |
| JsonErr | `0000000001878f60` | 128 |
| JsonGet | `0000000001878fe0` | 416 |
| JsonGetBool | `0000000001879180` | 128 |
| JsonGetInt | `0000000001879200` | 128 |
| JsonGetIntList | `0000000001879280` | 128 |
| JsonGetStr | `0000000001879300` | 128 |
| JsonGetStrList | `0000000001879380` | 128 |
| JsonOkMessage | `0000000001879400` | 128 |
| JsonOkResult | `0000000001879480` | 128 |
| JsonTimeout | `0000000001879500` | 736 |
| Prepare | `00000000018797e0` | 1664 |
| SessionRegenerateID | `0000000001879e60` | 992 |
| URLFor | `000000000187a240` | 192 |
| XSRFFormHTML | `000000000187a300` | ? |

## ScreenshotController

| 方法 | 地址 | 字节数 |
|---|---|---|
| JsonErr | `000000000187b580` | 128 |
| JsonGet | `000000000187b600` | 416 |
| JsonGetBool | `000000000187b7a0` | 128 |
| JsonGetInt | `000000000187b820` | 128 |
| JsonGetIntList | `000000000187b8a0` | 128 |
| JsonGetStr | `000000000187b920` | 128 |
| JsonGetStrList | `000000000187b9a0` | 128 |
| JsonOkMessage | `000000000187ba20` | 128 |
| JsonOkResult | `000000000187baa0` | 128 |
| JsonTimeout | `000000000187bb20` | 736 |
| Prepare | `000000000187be00` | 1664 |
| SessionRegenerateID | `000000000187c480` | 992 |
| URLFor | `000000000187c860` | 192 |
| XSRFFormHTML | `000000000187c920` | ? |

## SettingController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Edit | `00000000018374a0` | ? |
| JsonErr | `000000000187dba0` | 128 |
| JsonGet | `000000000187dc20` | 416 |
| JsonGetBool | `000000000187ddc0` | 128 |
| JsonGetInt | `000000000187de40` | 128 |
| JsonGetIntList | `000000000187dec0` | 128 |
| JsonGetStr | `000000000187df40` | 128 |
| JsonGetStrList | `000000000187dfc0` | 128 |
| JsonOkMessage | `000000000187e040` | 128 |
| JsonOkResult | `000000000187e0c0` | 128 |
| JsonTimeout | `000000000187e140` | 736 |
| Prepare | `000000000187e420` | 1664 |
| SessionRegenerateID | `000000000187eaa0` | 992 |
| URLFor | `000000000187ee80` | 192 |
| XSRFFormHTML | `000000000187ef40` | ? |

## TerminalController

| 方法 | 地址 | 字节数 |
|---|---|---|
| Ws | `0000000001837680` | 1696 |
| Shell | `0000000001837d20` | ? |
| JsonErr | `0000000001880220` | 128 |
| JsonGet | `00000000018802a0` | 416 |
| JsonGetBool | `0000000001880440` | 128 |
| JsonGetInt | `00000000018804c0` | 128 |
| JsonGetIntList | `0000000001880540` | 128 |
| JsonGetStr | `00000000018805c0` | 128 |
| JsonGetStrList | `0000000001880640` | 128 |
| JsonOkMessage | `00000000018806c0` | 128 |
| JsonOkResult | `0000000001880740` | 128 |
| JsonTimeout | `00000000018807c0` | 736 |
| Prepare | `0000000001880aa0` | 1664 |
| SessionRegenerateID | `0000000001881120` | 992 |
| URLFor | `0000000001881500` | 192 |
| XSRFFormHTML | `00000000018815c0` | ? |

## TunnelController

| 方法 | 地址 | 字节数 |
|---|---|---|
| List | `00000000018381e0` | 5632 |
| EditRemark | `00000000018397e0` | 256 |
| DelList | `00000000018398e0` | 288 |
| Del | `0000000001839a00` | 256 |
| Stop | `0000000001839b00` | 160 |
| Start | `0000000001839ba0` | 256 |
| Edit | `0000000001839ca0` | 1568 |
| Add | `000000000183a2c0` | ? |
| JsonErr | `00000000018828a0` | 128 |
| JsonGet | `0000000001882920` | 416 |
| JsonGetBool | `0000000001882ac0` | 128 |
| JsonGetInt | `0000000001882b40` | 128 |
| JsonGetIntList | `0000000001882bc0` | 128 |
| JsonGetStr | `0000000001882c40` | 128 |
| JsonGetStrList | `0000000001882cc0` | 128 |
| JsonOkMessage | `0000000001882d40` | 128 |
| JsonOkResult | `0000000001882dc0` | 128 |
| JsonTimeout | `0000000001882e40` | 736 |
| Prepare | `0000000001883120` | 1664 |
| SessionRegenerateID | `00000000018837a0` | 992 |
| URLFor | `0000000001883b80` | 192 |
| XSRFFormHTML | `0000000001883c40` | ? |
