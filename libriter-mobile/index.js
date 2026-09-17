// Vstupní bod aplikace.
//
// Přehrávací služba se musí zaregistrovat dřív, než se vykreslí první
// obrazovka – jinak by ovládání na zamčené obrazovce po studeném startu
// nefungovalo. Import expo-routeru je proto `require` až za registrací:
// statické importy se vyhodnocují jako první a router by naběhl dřív.
import TrackPlayer from 'react-native-track-player'

import { PlaybackService } from './src/player/service'

TrackPlayer.registerPlaybackService(() => PlaybackService)

require('expo-router/entry')
