# 基于官方 Elasticsearch 镜像
FROM docker.elastic.co/elasticsearch/elasticsearch:7.13.0

# 复制预先下载的 IK 插件到容器
COPY elasticsearch-analysis-ik-7.13.0.zip /tmp/

# 切换到 elasticsearch 用户安装插件
USER elasticsearch

# 从本地文件安装 IK 插件
RUN bin/elasticsearch-plugin install --batch file:///tmp/elasticsearch-analysis-ik-7.13.0.zip

# 清理临时文件
USER root
RUN rm -f /tmp/elasticsearch-analysis-ik-7.13.0.zip

# 切换回 elasticsearch 用户
USER elasticsearch

