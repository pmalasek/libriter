// Metro v npm workspaces.
//
// Závislosti se instalují v kořeni repozitáře a hoisting je rozhází mezi
// kořenové a lokální node_modules. Dvě kopie Reactu v jednom bundlu znamenají
// rozbité hooky, proto se hledání zužuje na dvě cesty v pevném pořadí: to, co
// má mobil u sebe (React ve verzi pinované Expem), vyhrává nad kořenem, kde
// bydlí React webového rozhraní.

const { getDefaultConfig } = require('expo/metro-config')
const path = require('node:path')

const projectRoot = __dirname
const workspaceRoot = path.resolve(projectRoot, '..')

const config = getDefaultConfig(projectRoot)

// libriter-shared se publikuje jako TypeScript zdroj, takže ho Metro musí
// sledovat stejně jako kód aplikace.
config.watchFolders = [workspaceRoot]

config.resolver.nodeModulesPaths = [
  path.resolve(projectRoot, 'node_modules'),
  path.resolve(workspaceRoot, 'node_modules'),
]
// Bez tohohle by Metro šplhalo adresáři nahoru a našlo si i balíčky, které
// mobilu nepatří (třeba React z webu).
config.resolver.disableHierarchicalLookup = true

module.exports = config
