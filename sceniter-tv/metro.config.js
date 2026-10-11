// Metro pro TV klienta.
//
// TV běží na forku react-native-tvos, mobil Libriteru na běžném React
// Native – proto sceniter-tv není npm workspace a má vlastní node_modules.
// Sdílené balíčky (sceniter-shared, core-shared) se berou přímo ze zdrojů
// v monorepu a všechny jejich závislosti (i18next…) se řeší výhradně
// z sceniter-tv/node_modules, ať je v bundlu jediná kopie Reactu i i18next.

const path = require('path')
const { getDefaultConfig } = require('expo/metro-config')

const root = path.resolve(__dirname, '..')
const shared = {
  'sceniter-shared': path.join(root, 'sceniter-shared/src/index.ts'),
  'core-shared': path.join(root, 'core-shared/src/index.ts'),
}

const config = getDefaultConfig(__dirname)
config.watchFolders = [path.join(root, 'sceniter-shared'), path.join(root, 'core-shared')]
config.resolver.nodeModulesPaths = [path.join(__dirname, 'node_modules')]
config.resolver.disableHierarchicalLookup = true

const upstream = config.resolver.resolveRequest
config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (shared[moduleName]) return { type: 'sourceFile', filePath: shared[moduleName] }
  return (upstream ?? context.resolveRequest)(context, moduleName, platform)
}

module.exports = config
