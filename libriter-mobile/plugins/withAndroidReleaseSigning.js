const { withAppBuildGradle } = require('@expo/config-plugins')

/**
 * Podepisování release buildu Androidu vlastním klíčem.
 *
 * Šablona Expa podepisuje release build debug klíčem, se kterým aplikace do
 * Google Play neprojde a jinak podepsaná aktualizace se přes ni nenainstaluje.
 * Plugin přidá `signingConfigs.release`, který čte klíč z Gradle properties:
 *
 *   LIBRITER_UPLOAD_STORE_FILE, LIBRITER_UPLOAD_STORE_PASSWORD,
 *   LIBRITER_UPLOAD_KEY_ALIAS, LIBRITER_UPLOAD_KEY_PASSWORD
 *
 * `just mobile-keystore` je zapíše do ~/.gradle/gradle.properties, takže klíč
 * ani hesla nejsou v repozitáři. Když properties chybí, release build se dál
 * podepíše debug klíčem – vývojářský `just mobile-apk` tak funguje i bez něj.
 */

const MARKER = 'LIBRITER_UPLOAD_STORE_FILE'

const RELEASE_SIGNING_CONFIG = `
        if (findProperty('LIBRITER_UPLOAD_STORE_FILE')) {
            release {
                storeFile file(findProperty('LIBRITER_UPLOAD_STORE_FILE'))
                storePassword findProperty('LIBRITER_UPLOAD_STORE_PASSWORD')
                keyAlias findProperty('LIBRITER_UPLOAD_KEY_ALIAS')
                keyPassword findProperty('LIBRITER_UPLOAD_KEY_PASSWORD')
            }
        }`

function addReleaseSigning(gradle) {
  // prebuild bez --clean pouští pluginy znovu nad už upraveným souborem
  if (gradle.includes(MARKER)) return gradle

  const withConfig = gradle.replace(/(signingConfigs\s*\{)/, `$1${RELEASE_SIGNING_CONFIG}`)
  const withBuildType = withConfig.replace(
    /(release\s*\{[^}]*?)signingConfig signingConfigs\.debug/,
    `$1signingConfig findProperty('${MARKER}') ? signingConfigs.release : signingConfigs.debug`,
  )
  if (withBuildType === withConfig || withConfig === gradle) {
    throw new Error('withAndroidReleaseSigning: app/build.gradle nemá očekávaný tvar, plugin je potřeba upravit')
  }
  return withBuildType
}

module.exports = function withAndroidReleaseSigning(config) {
  return withAppBuildGradle(config, (cfg) => {
    cfg.modResults.contents = addReleaseSigning(cfg.modResults.contents)
    return cfg
  })
}
