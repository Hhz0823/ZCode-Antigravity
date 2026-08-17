import AppKit

let output = CommandLine.arguments.dropFirst().first ?? "AppIcon-1024.png"
let size = NSSize(width: 1024, height: 1024)
let image = NSImage(size: size)
image.lockFocus()

let bounds = NSRect(origin: .zero, size: size)
let background = NSGradient(colors: [
    NSColor(calibratedRed: 0.035, green: 0.15, blue: 0.36, alpha: 1),
    NSColor(calibratedRed: 0.02, green: 0.42, blue: 0.96, alpha: 1)
])!
let path = NSBezierPath(roundedRect: bounds.insetBy(dx: 54, dy: 54), xRadius: 220, yRadius: 220)
background.draw(in: path, angle: -48)

NSGraphicsContext.current?.saveGraphicsState()
path.addClip()
for index in 0..<7 {
    let inset = CGFloat(120 + index * 62)
    let ring = NSBezierPath(ovalIn: bounds.insetBy(dx: inset, dy: inset))
    ring.lineWidth = 12
    NSColor.white.withAlphaComponent(0.06 + CGFloat(index) * 0.012).setStroke()
    ring.stroke()
}
NSGraphicsContext.current?.restoreGraphicsState()

let plate = NSBezierPath(roundedRect: bounds.insetBy(dx: 180, dy: 180), xRadius: 132, yRadius: 132)
NSColor.white.withAlphaComponent(0.12).setFill()
plate.fill()
NSColor.white.withAlphaComponent(0.46).setStroke()
plate.lineWidth = 10
plate.stroke()

let paragraph = NSMutableParagraphStyle()
paragraph.alignment = .center
let attributes: [NSAttributedString.Key: Any] = [
    .font: NSFont.systemFont(ofSize: 248, weight: .bold),
    .foregroundColor: NSColor.white,
    .paragraphStyle: paragraph,
    .kern: -12
]
("ZA" as NSString).draw(in: NSRect(x: 174, y: 350, width: 676, height: 300), withAttributes: attributes)

image.unlockFocus()
guard let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fatalError("Unable to render app icon")
}
try png.write(to: URL(fileURLWithPath: output), options: .atomic)
