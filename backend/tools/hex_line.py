# -*- coding: utf-8 -*-
import ftfy

line = "鍒涘缓 etcd resolver 澶辫触"
print([hex(ord(c)) for c in line])
print(ftfy.fix_text(line))
