# Stress-ng Installation Ansible Playbook

## 目录结构
```
├── install-stress-ng.yml    # 主playbook文件
├── inventory.ini           # 服务器清单文件
├── files/                 # 存放RPM包和源码包的目录
│   ├── stress-ng-0.17.08-1.el7.x86_64.rpm  # RPM安装包
│   └── stress-ng-0.17.08.tar.gz            # 源码包
└── README-ansible.md      # 使用说明
```

## 使用前准备

### 1. 准备文件包
在 `files/` 目录下放置以下文件：

**RPM包** (优先尝试)：
```bash
# 下载适合你系统的RPM包，例如：
wget -O files/stress-ng-0.17.08-1.el7.x86_64.rpm \
  https://download-ib01.fedoraproject.org/pub/epel/7/x86_64/Packages/s/stress-ng-0.17.08-1.el7.x86_64.rpm
```

**源码包** (最后备选)：
```bash
# 下载源码包
wget -O files/stress-ng-0.17.08.tar.gz \
  https://github.com/ColinIanKing/stress-ng/archive/V0.17.08.tar.gz
```

### 2. 配置服务器清单
编辑 `inventory.ini` 文件，添加你的目标服务器：

```ini
[stress_servers]
server1 ansible_host=192.168.1.10 ansible_user=root
server2 ansible_host=192.168.1.11 ansible_user=root
```

## 运行playbook

### 基本运行
```bash
ansible-playbook -i inventory.ini install-stress-ng.yml
```

### 使用密码认证
```bash
ansible-playbook -i inventory.ini install-stress-ng.yml --ask-pass
```

### 使用SSH密钥
```bash
ansible-playbook -i inventory.ini install-stress-ng.yml --private-key=/path/to/your/key
```

### 指定特定主机
```bash
ansible-playbook -i inventory.ini install-stress-ng.yml --limit server1
```

## 安装逻辑

Playbook会按以下顺序尝试安装stress-ng：

1. **方法1: YUM/DNF安装**
   - 尝试通过系统包管理器安装
   - 最简单快捷的方法

2. **方法2: RPM包安装**
   - 如果yum失败，使用files/目录下的RPM包
   - 适用于有网络限制的环境

3. **方法3: 源码编译安装**
   - 如果RPM也失败，编译源码安装
   - 会自动安装编译依赖

## 验证安装

安装完成后会自动验证：
```bash
# 在目标服务器上运行
stress-ng --version
```

## 故障排除

### 常见问题

1. **权限问题**
   - 确保ansible用户有sudo权限
   - 或直接使用root用户

2. **网络问题**
   - 确保能SSH连接到目标服务器
   - 检查防火墙设置

3. **依赖包缺失**
   - playbook会尝试安装编译依赖
   - 可能需要手动安装某些基础包

### 调试运行
```bash
# 增加详细输出
ansible-playbook -i inventory.ini install-stress-ng.yml -v

# 更详细的调试信息
ansible-playbook -i inventory.ini install-stress-ng.yml -vvv
```

## 自定义变量

可以在playbook中修改以下变量：
- `stress_ng_version`: stress-ng版本号
- `stress_ng_rpm_file`: RPM包文件名
- `stress_ng_source_file`: 源码包文件名
- `temp_dir`: 临时目录路径