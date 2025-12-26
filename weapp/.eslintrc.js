module.exports = {
  root: true,
  env: {
    browser: true,
    es6: true,
    node: true,
  },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 2020,
    sourceType: 'module',
    project: './tsconfig.json',
  },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
  ],
  plugins: ['@typescript-eslint'],
  rules: {
    // 禁止显式 any 类型 - 渐进式迁移,暂时设为警告
    '@typescript-eslint/no-explicit-any': 'warn',
    
    // 要求函数必须有返回类型注解 - 新代码强制,旧代码警告
    '@typescript-eslint/explicit-function-return-type': ['off', {
      allowExpressions: true,
      allowTypedFunctionExpressions: true,
      allowHigherOrderFunctions: true,
    }],
    
    // 要求类方法必须有返回类型 - 关闭,逐步迁移
    '@typescript-eslint/explicit-module-boundary-types': 'off',
    
    // 禁止 var 声明
    'no-var': 'error',
    
    // 优先使用 const
    'prefer-const': 'warn',
    
    // 禁止未使用的变量 - 降级为警告
    '@typescript-eslint/no-unused-vars': ['warn', {
      argsIgnorePattern: '^_',
      varsIgnorePattern: '^_',
    }],
    
    // 禁止非空断言 - 关闭,小程序框架经常需要
    '@typescript-eslint/no-non-null-assertion': 'off',
    
    // 要求使用模板字符串
    'prefer-template': 'off',
    
    // 禁止 require 语句(非 import) - 小程序动态加载需要
    '@typescript-eslint/no-var-requires': 'off',
    
    // 对象简写
    'object-shorthand': 'warn',
    
    // 箭头函数参数括号
    'arrow-parens': ['warn', 'always'],
    
    // 强制使用分号
    'semi': ['error', 'never'],
    '@typescript-eslint/member-delimiter-style': ['error', {
      multiline: {
        delimiter: 'none',
      },
      singleline: {
        delimiter: 'comma',
      },
    }],
    
    // 缩进 - 关闭,保持现有代码风格(4空格)
    'indent': 'off',
    '@typescript-eslint/indent': 'off',
    
    // 引号使用单引号
    'quotes': ['error', 'single', {
      avoidEscape: true,
      allowTemplateLiterals: true,
    }],
    
    // 逗号风格
    'comma-dangle': ['error', {
      arrays: 'never',
      objects: 'never',
      imports: 'never',
      exports: 'never',
      functions: 'never',
    }],
    
    // 对象花括号间距
    'object-curly-spacing': ['error', 'always'],
    
    // 数组方括号间距
    'array-bracket-spacing': ['error', 'never'],
    
    // 注释空格
    'spaced-comment': ['warn', 'always'],
    
    // 要求使用 === 和 !==
    'eqeqeq': ['error', 'always'],
    
    // 禁止 console(生产环境)
    'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
    
    // 禁止 debugger(生产环境)
    'no-debugger': process.env.NODE_ENV === 'production' ? 'error' : 'off',
  },
  
  // 忽略某些文件
  ignorePatterns: [
    'miniprogram_npm/**',
    'libs/**',
    'scripts/**',
    '*.js', // 忽略根目录的 .js 配置文件
    '!.eslintrc.js',
  ],
  
  // 全局变量(微信小程序)
  globals: {
    wx: 'readonly',
    App: 'readonly',
    Page: 'readonly',
    Component: 'readonly',
    getApp: 'readonly',
    getCurrentPages: 'readonly',
    requirePlugin: 'readonly',
    WechatMiniprogram: 'readonly',
  },
}
