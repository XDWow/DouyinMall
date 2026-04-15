# -*- coding: utf-8 -*-
"""Fix common UTF-8 mojibake in Go source (GBK/GB18030 mis-decoded as UTF-8)."""
from __future__ import annotations

import os
import re
import sys

# Known phrase-level fixes (substring replace, longest first)
PHRASE_FIXES: list[tuple[str, str]] = [
    # wire / ioc / grpc boilerplate
    ("鍒涘缓 etcd 娉ㄥ唽涓績澶辫触", "创建 etcd 注册中心失败"),
    ("鍒涘缓 etcd 娉ㄥ唽涓績", "创建 etcd 注册中心"),
    ("鍒濆鍖?etcd 娉ㄥ唽涓績", "初始化 etcd 注册中心"),
    ("鍒濆鍖?etcd 娉ㄥ唽涓績", "初始化 etcd 注册中心"),
    ("鍒涘缓 Kitex 鏈嶅姟", "创建 Kitex 服务"),
    ("鍒涘缓 etcd resolver 澶辫触", "创建 etcd resolver 失败"),
    ("鍒涘缓 etcd resolver澶辫触", "创建 etcd resolver 失败"),
    ("鍒涘缓 User RPC 瀹㈡埛绔け璐?direct)", "创建 User RPC 客户端失败(direct)"),
    ("鍒涘缓 User RPC 瀹㈡埛绔け璐?", "创建 User RPC 客户端失败"),
    ("鍒涘缓zap鏃ュ織鏍稿績", "创建 zap 日志核心"),
    ("鍒涘缓 Canal Producer 澶辫触", "初始化 Canal Producer 失败"),
    ("鍒涘缓 Canal 澶辫触", "创建 Canal 失败"),
    ("鍒濆鍖?Kafka Client 澶辫触", "初始化 Kafka Client 失败"),
    ("鍒濆鍖?Kafka SyncProducer 澶辫触", "初始化 Kafka SyncProducer 失败"),
    ("鍒濆鍖?Kafka Client", "初始化 Kafka Client"),
    ("鍒濆鍖?Kafka SyncProducer", "初始化 Kafka SyncProducer"),
    ("鍒濆鍖?Canal Producer 澶辫触", "初始化 Canal Producer 失败"),
    ("InitApp 鍒濆鍖栨暣涓簲鐢?", "InitApp 初始化整个应用"),
    ("鍒濆鍖栨暣涓簲鐢?", "初始化整个应用"),
    ("鍩虹璁炬柦灞傦紙ioc 鍖呮彁渚涳級", "基础设施层（ioc 包提供）"),
    ("DAO 灞?", "DAO 层"),
    ("Cache 灞?", "Cache 层"),
    ("Repository 灞?", "Repository 层"),
    ("Service 灞?", "Service 层"),
    ("Handler 灞?", "Handler 层"),
    ("浠撳偍", "仓储"),
    ("鍒涘缓", "创建"),
    ("鍒濆鍖?", "初始化"),
    ("鍒濆鍖", "初始化"),
    ("澶辫触", "失败"),
    ("瀹㈡埛绔?", "客户端"),
    ("瀹㈡埛绔", "客户端"),
    ("鍟嗗搧", "商品"),
    ("鍟嗗", "商家"),
    ("璁㈠崟", "订单"),
    ("搴撳瓨", "库存"),
    ("浼樻儬鍒?", "优惠券"),
    ("浼樻儬", "优惠"),
    ("绱㈠紩", "索引"),
    ("鎼滅储", "搜索"),
    ("閫傜敤", "适用"),
    ("鏄惁", "是否"),
    ("瑙ｆ瀽", "解析"),
    ("鎵归噺", "批量"),
    ("鍙戦€?", "发送"),
    ("鍙戦€", "发送"),
    ("璇诲彇", "读取"),
    ("閰嶇疆", "配置"),
    ("鏂囦欢", "文件"),
    ("鍚姩", "启动"),
    ("鍏抽棴", "关闭"),
    ("璀﹀憡", "警告"),
    ("榛樿璁", "默认"),
    ("浜嬪姟", "事务"),
    ("鍥炴粴", "回滚"),
    ("鎻愪氦", "提交"),
    ("娉ㄥ唽", "注册"),
    ("鐧诲綍", "登录"),
    ("鏌ヨ", "查询"),
    ("鍙栨秷", "取消"),
    ("鏇存柊", "更新"),
    ("鍒犻櫎", "删除"),
    ("娴嬭瘯", "测试"),
    ("澶辫触", "失败"),
    ("澶辫", "失败"),
    ("鍒犻", "删除"),
    ("缂撳瓨", "缓存"),
    ("鍛戒腑", "命中"),
    ("鏈懡涓", "未命中"),
    ("鍥炲～", "回填"),
    ("鍩烘湰淇℃伅", "基本信息"),
    ("鍒涘缓鎴愬姛", "创建成功"),
    ("鏇存柊澶辫触", "更新失败"),
    ("涓嶅垹缂撳瓨", "不删缓存"),
    ("鐑偣鏁版嵁", "热点数据"),
    ("鍟嗗搧1", "商品1"),
    ("鍟嗗搧2", "商品2"),
    ("鍟嗗搧3", "商品3"),
    ("鍟嗗搧41", "商品41"),
    ("鍟嗗搧宸蹭笅鏋?", "商品已下架"),
    ("搴撳瓨涓嶈冻", "库存不足"),
    ("鍟嗗搧涓嶅瓨鍦?", "商品不存在"),
    ("鍟嗗搧涓嶈兘鍐嶅噺灏戜簡", "商品数量不能再减少了"),
    ("鍒濆鍖栨湇鍔＄粍浠?", "初始化服务组件"),
    ("鍒犻櫎鍟嗗搧", "删除商品"),
    ("璇诲彇閰嶇疆鏂囦欢澶辫触", "读取配置文件失败"),
    ("gRPC鏈嶅姟鍚姩澶辫触", "gRPC 服务启动失败"),
    ("gRPC鏈嶅姟鍏抽棴澶辫触", "gRPC 服务关闭失败"),
    ("浣跨敤榛樿璁ら厤缃", "使用默认配置"),
    ("鍚姩Kafka娑堣垂鑰咃紙璁㈠崟鐘舵€佸彉鏇达級", "启动 Kafka 消费者（订单状态变更）"),
]

# Sort by length descending so longer phrases win
PHRASE_FIXES.sort(key=lambda x: len(x[0]), reverse=True)


def apply_phrase_fixes(text: str) -> str:
    out = text
    for bad, good in PHRASE_FIXES:
        out = out.replace(bad, good)
    return out


def try_gb18030_roundtrip(s: str) -> str | None:
    """If s is UTF-8 misread as GB18030, re-encode to GB18030 bytes and decode as UTF-8."""
    try:
        b = s.encode("gb18030", errors="strict")
        return b.decode("utf-8", errors="strict")
    except (UnicodeEncodeError, UnicodeDecodeError):
        return None


def fix_line(line: str) -> str:
    line2 = apply_phrase_fixes(line)
    # Only attempt full-line GB18030 fix for lines that still look corrupted
    if re.search(r"[鍒閸鐟缂鍟瀹澶鎼璁搴閫鏄瑙鎵鍙璇閰鏂鍚鍏璀榛浜鍥鎻娉鐧鏌鍙]", line2):
        # Try segment-wise: split by ASCII runs, fix Chinese-only segments
        parts = re.split(r"([ -~]+)", line2)
        new_parts = []
        for p in parts:
            if not p or re.fullmatch(r"[ -~]+", p):
                new_parts.append(p)
                continue
            if re.search(r"[鍒閸鐟缂鍟瀹澶鎼璁搴]", p):
                t = try_gb18030_roundtrip(p)
                if t and not re.search(r"[\ufffd]", t):
                    new_parts.append(t)
                else:
                    new_parts.append(p)
            else:
                new_parts.append(p)
        line2 = "".join(new_parts)
    return line2


def should_process(path: str) -> bool:
    if "vendor" in path.replace("\\", "/"):
        return False
    if path.endswith("fix_mojibake.py"):
        return False
    return path.endswith(".go")


def main() -> int:
    root = os.path.join(os.path.dirname(__file__), "..")
    root = os.path.normpath(root)
    changed = 0
    for dirpath, _, filenames in os.walk(root):
        for fn in filenames:
            path = os.path.join(dirpath, fn)
            if not should_process(path):
                continue
            try:
                with open(path, "r", encoding="utf-8") as f:
                    orig = f.read()
            except OSError:
                continue
            # Skip binary / huge
            if "\x00" in orig or len(orig) > 2_000_000:
                continue
            lines = orig.splitlines(keepends=True)
            new_lines = [fix_line(L) for L in lines]
            new = "".join(new_lines)
            if new != orig:
                with open(path, "w", encoding="utf-8", newline="") as f:
                    f.write(new)
                changed += 1
                print("fixed:", path)
    print("total files changed:", changed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
