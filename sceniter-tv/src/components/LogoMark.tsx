import Svg, { Defs, LinearGradient, Path, Stop } from 'react-native-svg'
import { colors } from '@/theme'

/**
 * Značka Sceniteru: zaoblený trojúhelník „přehrát“ v kovovém přechodu
 * tématu – stejná kresba jako na webu (sceniter-frontend Logo.tsx).
 */
export function LogoMark({ size }: { size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 32 32" fill="none" accessibilityLabel="Sceniter">
      <Defs>
        <LinearGradient id="sceniterMark" x1="0.1" y1="0" x2="0.9" y2="1">
          <Stop offset="0" stopColor={colors.accentLight} />
          <Stop offset="0.5" stopColor={colors.accent} />
          <Stop offset="1" stopColor={colors.accentDeep} />
        </LinearGradient>
      </Defs>
      <Path
        d="M8.5 6.2c0-1.5 1.6-2.4 2.9-1.7l15.1 9.8c1.2.8 1.2 2.6 0 3.4L11.4 27.5c-1.3.8-2.9-.2-2.9-1.7Z"
        stroke="url(#sceniterMark)"
        strokeWidth={3.6}
        strokeLinejoin="round"
      />
      <Path d="M13.2 12.4v7.2c0 .5.5.8.9.5l5.6-3.6c.4-.3.4-.8 0-1.1l-5.6-3.6c-.4-.3-.9 0-.9.6Z" fill="url(#sceniterMark)" />
    </Svg>
  )
}
