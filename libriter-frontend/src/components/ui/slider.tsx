"use client"

import * as React from "react"
import { cn } from "cn"
import { Slider as SliderPrimitive } from "radix-ui"

function Slider({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root> & {
  /** `lg` je pro dotyk – silnější dráha a větší úchyt. */
  size?: "default" | "lg"
}) {
  const large = size === "lg"
  return (
    <SliderPrimitive.Root
      data-slot="slider"
      className={cn(
        "relative flex w-full touch-none items-center select-none data-disabled:opacity-50 data-[orientation=vertical]:h-full data-[orientation=vertical]:w-auto data-[orientation=vertical]:flex-col",
        className
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className={cn(
          "relative grow overflow-hidden rounded-full bg-muted data-[orientation=horizontal]:w-full data-[orientation=vertical]:h-full",
          large
            ? "data-[orientation=horizontal]:h-2.5 data-[orientation=vertical]:w-2.5"
            : "data-[orientation=horizontal]:h-1.5 data-[orientation=vertical]:w-1.5"
        )}
      >
        <SliderPrimitive.Range
          data-slot="slider-range"
          className="absolute bg-primary data-[orientation=horizontal]:h-full data-[orientation=vertical]:w-full"
        />
      </SliderPrimitive.Track>
      <SliderPrimitive.Thumb
        data-slot="slider-thumb"
        className={cn(
          "block shrink-0 rounded-full border border-primary/50 bg-background shadow-sm transition-[color,box-shadow] hover:ring-4 hover:ring-ring/30 focus-visible:ring-4 focus-visible:ring-ring/50 focus-visible:outline-hidden disabled:pointer-events-none",
          large ? "size-5" : "size-3.5"
        )}
      />
    </SliderPrimitive.Root>
  )
}

export { Slider }
