module.exports = {
  outputDir: '../static/dist',
  productionSourceMap: false,
  css: {
    extract: true
  },
  chainWebpack: function (config) {
    config.entry('app').clear().add('./src/main.js')
  }
}
