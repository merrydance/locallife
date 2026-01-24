module.exports = {
  apps: [
    {
      name: 'my-next-app',
      script: 'node_modules/next/dist/bin/next',
      args: 'start',
      instances: 2, // 开启集群模式，参数可以是"max"，表示充分利用 CPU
      exec_mode: 'cluster',
      env: {
        NODE_ENV: 'production',
        PORT: 3000
      }
    }
  ]
}