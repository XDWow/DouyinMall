# -*- coding: utf-8 -*-
import ftfy

tests = [
    "// InitApp 鍒濆鍖栨暣涓簲鐢?",
    "鍟嗗搧",
    "鍒涘缓 etcd resolver 澶辫触",
    "娴ｇ姵妲搁幎鏍叾閸熷棗鐓勯惃?AI 鐎广垺婀囬崝鈺傚",
]
for t in tests:
    out = ftfy.fix_text(t)
    print("IN :", t[:70])
    print("OUT:", out[:70])
    print("---")
