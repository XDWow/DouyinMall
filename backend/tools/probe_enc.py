# -*- coding: utf-8 -*-
# Garbled UTF-8 strings as they appear in repo (创建/商品/订单/失败)
samples = ["\u9342\u51d7\u7f13", "\u935f\u55e7\u640d", "\u7421\u4e20\u5d19", "\u6d9e\u8fa9\u89e6"]
for s in samples:
    for name, enc, dec in [
        ("gbk_enc_utf8_dec", "gbk", "utf-8"),
        ("utf8_enc_gbk_dec", "utf-8", "gbk"),
    ]:
        try:
            b = s.encode(enc)
            t = b.decode(dec)
            print(repr(s), name, "->", repr(t))
        except Exception as e:
            print(repr(s), name, e)
