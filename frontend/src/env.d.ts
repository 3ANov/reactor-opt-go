declare module 'plotly.js-dist-min' {
  const Plotly: {
    react(
      element: HTMLElement,
      data: Record<string, unknown>[],
      layout: Record<string, unknown>,
      config: Record<string, unknown>,
    ): Promise<void>
    purge(element: HTMLElement): void
    Plots: { resize(element: HTMLElement): Promise<void> }
  }
  export default Plotly
}
