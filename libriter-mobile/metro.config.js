// Metro v npm workspaces.
//
// Od Expo SDK 52 getDefaultConfig monorepo detekuje sám (watchFolders
// i nodeModulesPaths), takže stačí výchozí konfigurace. Jediná kopie Reactu
// je zajištěná tím, že web i mobil pinují stejnou verzi (tu, kterou chce Expo).

const { getDefaultConfig } = require('expo/metro-config')

module.exports = getDefaultConfig(__dirname)
