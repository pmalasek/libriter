import {
  AtomIcon,
  BabyIcon,
  BirdIcon,
  BookOpenIcon,
  CameraIcon,
  CarIcon,
  CastleIcon,
  ClapperboardIcon,
  DnaIcon,
  DramaIcon,
  EarthIcon,
  FilmIcon,
  GhostIcon,
  GraduationCapIcon,
  HeartIcon,
  LandmarkIcon,
  LaughIcon,
  MicIcon,
  MicroscopeIcon,
  MonitorPlayIcon,
  MountainIcon,
  MusicIcon,
  PawPrintIcon,
  PlaneIcon,
  PopcornIcon,
  ProjectorIcon,
  RocketIcon,
  SparklesIcon,
  StarIcon,
  SwordsIcon,
  TrophyIcon,
  TvIcon,
  VideoIcon,
  type LucideIcon,
} from 'lucide-react'
import type { Library } from 'sceniter-shared'

/**
 * Ikony, ze kterých admin vybírá ikonu knihovny. Klíč se ukládá na serveru –
 * přejmenování nebo odebrání klíče vrátí knihovně výchozí ikonu.
 */
export const LIBRARY_ICONS: Record<string, LucideIcon> = {
  film: FilmIcon,
  clapperboard: ClapperboardIcon,
  popcorn: PopcornIcon,
  projector: ProjectorIcon,
  tv: TvIcon,
  'monitor-play': MonitorPlayIcon,
  video: VideoIcon,
  camera: CameraIcon,
  drama: DramaIcon,
  laugh: LaughIcon,
  heart: HeartIcon,
  ghost: GhostIcon,
  rocket: RocketIcon,
  sparkles: SparklesIcon,
  swords: SwordsIcon,
  castle: CastleIcon,
  landmark: LandmarkIcon,
  earth: EarthIcon,
  mountain: MountainIcon,
  'paw-print': PawPrintIcon,
  bird: BirdIcon,
  microscope: MicroscopeIcon,
  atom: AtomIcon,
  dna: DnaIcon,
  'graduation-cap': GraduationCapIcon,
  'book-open': BookOpenIcon,
  baby: BabyIcon,
  music: MusicIcon,
  mic: MicIcon,
  trophy: TrophyIcon,
  car: CarIcon,
  plane: PlaneIcon,
  star: StarIcon,
}

/** Výchozí ikona podle typu knihovny. */
export function defaultLibraryIcon(kind: Library['kind']): LucideIcon {
  return kind === 'show' ? TvIcon : FilmIcon
}

/** Ikona knihovny: vybraná, jinak podle typu. */
export function libraryIcon(lib: Pick<Library, 'kind' | 'icon'>): LucideIcon {
  return (lib.icon && LIBRARY_ICONS[lib.icon]) || defaultLibraryIcon(lib.kind)
}
