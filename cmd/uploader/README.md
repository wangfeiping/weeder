### 文件上传命令行工具

可批量上传
可上传整个目录

注意：gzip压缩上传仅支持少数已知文本文件，如：".txt", ".html", ".js", ".css"
例如：".go"则不支持gzip压缩上传

weeder api:
./uploader -d test.txt.gz -p /public/201807121619/

filer api:
./uploader -a "http://192.168.1.182:8888/public/201807121626/" -d test.txt.gz

master api:
./uploader -a "http://192.168.1.182:9333/submit" -d test.txt.gz -p /public/201807121632/
