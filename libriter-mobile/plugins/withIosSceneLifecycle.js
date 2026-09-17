const { IOSConfig, withAppDelegate, withInfoPlist, withXcodeProject } = require('@expo/config-plugins')
const fs = require('node:fs')
const path = require('node:path')

/**
 * Přechod na scénový životní cyklus UIKit (UIScene).
 *
 * Nejnovější iOS SDK při startu tvrdě spadne, pokud aplikace scénový životní
 * cyklus nepřijala („UIScene life cycle is required for apps built with this
 * SDK“). Expo SDK 58 už tak generuje projekt rovnou; SDK 57, na kterém tahle
 * aplikace stojí, ještě ne – ačkoliv třídu `ExpoAppSceneDelegate` v balíčku
 * `expo` už má. Tenhle plugin ten kousek doplní, přesně tak, jak to dělá
 * šablona SDK 58:
 *
 *  1. do Info.plist přidá `UIApplicationSceneManifest`,
 *  2. vytvoří `SceneDelegate.swift` a zařadí ho do Xcode projektu,
 *  3. z `AppDelegate` odstraní zakládání okna – to teď dělá scéna – a nechá
 *     ho hlásit se k `ExpoReactNativeFactoryProvider`, aby si scéna uměla
 *     říct o React Native factory.
 *
 * Až aplikace přejde na SDK 58, celý plugin se dá smazat i se zápisem
 * v app.json.
 */

const SCENE_DELEGATE_SWIFT = `internal import Expo

@objc(SceneDelegate)
class SceneDelegate: ExpoAppSceneDelegate {
  // Okno i start React Native obstarává ExpoAppSceneDelegate; tahle třída
  // existuje proto, že ji Info.plist musí umět pojmenovat.
}
`

/** Okno zakládá scéna, ne app delegate. */
const WINDOW_SETUP = `
#if os(iOS) || os(tvOS)
    window = UIWindow(frame: UIScreen.main.bounds)
    factory.startReactNative(
      withModuleName: "main",
      in: window,
      launchOptions: launchOptions)
#endif
`

const WINDOW_REPLACEMENT = `
    // Okno zakládá a React Native spouští SceneDelegate – scénový životní
    // cyklus to vyžaduje (viz plugins/withIosSceneLifecycle.js).
`

function withSceneManifest(config) {
  return withInfoPlist(config, (cfg) => {
    cfg.modResults.UIApplicationSceneManifest = {
      UIApplicationSupportsMultipleScenes: false,
      UISceneConfigurations: {
        UIWindowSceneSessionRoleApplication: [
          {
            UISceneConfigurationName: 'Default Configuration',
            UISceneDelegateClassName: '$(PRODUCT_MODULE_NAME).SceneDelegate',
          },
        ],
      },
    }
    return cfg
  })
}

function withSceneDelegateFile(config) {
  return withXcodeProject(config, (cfg) => {
    const projectName = IOSConfig.XcodeUtils.getProjectName(cfg.modRequest.projectRoot)
    const relativePath = `${projectName}/SceneDelegate.swift`
    const absolutePath = path.join(cfg.modRequest.platformProjectRoot, relativePath)

    fs.writeFileSync(absolutePath, SCENE_DELEGATE_SWIFT)

    // Soubor musí být i v projektu, ne jen na disku – jinak se nepřeloží
    // a Info.plist by odkazoval na třídu, která v binárce není.
    if (!cfg.modResults.hasFile(relativePath)) {
      IOSConfig.XcodeUtils.addBuildSourceFileToGroup({
        filepath: relativePath,
        groupName: projectName,
        project: cfg.modResults,
        projectRoot: cfg.modRequest.projectRoot,
      })
    }
    return cfg
  })
}

function withAppDelegateForScenes(config) {
  return withAppDelegate(config, (cfg) => {
    let contents = cfg.modResults.contents

    if (contents.includes(WINDOW_SETUP.trim())) {
      contents = contents.replace(WINDOW_SETUP, WINDOW_REPLACEMENT)
    }

    // Scéna si od app delegate vyžádá React Native factory; bez tohohle
    // protokolu skončí start fatalError uvnitř ExpoAppSceneDelegate.
    if (!contents.includes('ExpoReactNativeFactoryProvider')) {
      contents = contents.replace(
        'class AppDelegate: ExpoAppDelegate {',
        'class AppDelegate: ExpoAppDelegate, ExpoReactNativeFactoryProvider {',
      )
    }

    cfg.modResults.contents = contents
    return cfg
  })
}

module.exports = function withIosSceneLifecycle(config) {
  return withAppDelegateForScenes(withSceneDelegateFile(withSceneManifest(config)))
}
