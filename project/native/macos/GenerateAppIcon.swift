import AppKit

let output = CommandLine.arguments.dropFirst().first ?? "AppIcon-1024.png"
let canvas = NSRect(x: 0, y: 0, width: 1024, height: 1024)
let image = NSImage(size: canvas.size)

func fill(_ path: NSBezierPath, color: NSColor) {
    color.setFill()
    path.fill()
}

func sparkle(center: NSPoint, radius: CGFloat, inner: CGFloat) -> NSBezierPath {
    let path = NSBezierPath()
    path.move(to: NSPoint(x: center.x, y: center.y + radius))
    path.curve(to: NSPoint(x: center.x + radius, y: center.y), controlPoint1: NSPoint(x: center.x + inner, y: center.y + inner), controlPoint2: NSPoint(x: center.x + inner, y: center.y + inner))
    path.curve(to: NSPoint(x: center.x, y: center.y - radius), controlPoint1: NSPoint(x: center.x + inner, y: center.y - inner), controlPoint2: NSPoint(x: center.x + inner, y: center.y - inner))
    path.curve(to: NSPoint(x: center.x - radius, y: center.y), controlPoint1: NSPoint(x: center.x - inner, y: center.y - inner), controlPoint2: NSPoint(x: center.x - inner, y: center.y - inner))
    path.curve(to: NSPoint(x: center.x, y: center.y + radius), controlPoint1: NSPoint(x: center.x - inner, y: center.y + inner), controlPoint2: NSPoint(x: center.x - inner, y: center.y + inner))
    path.close()
    return path
}

image.lockFocus()
NSGraphicsContext.current?.imageInterpolation = .high

let tile = NSBezierPath(roundedRect: canvas.insetBy(dx: 48, dy: 48), xRadius: 224, yRadius: 224)
let tileShadow = NSShadow()
tileShadow.shadowColor = NSColor.black.withAlphaComponent(0.28)
tileShadow.shadowBlurRadius = 38
tileShadow.shadowOffset = NSSize(width: 0, height: -18)
NSGraphicsContext.current?.saveGraphicsState()
tileShadow.set()
NSColor(calibratedRed: 1.0, green: 0.78, blue: 0.03, alpha: 1).setFill()
tile.fill()
NSGraphicsContext.current?.restoreGraphicsState()

let yellow = NSGradient(colors: [
    NSColor(calibratedRed: 1.0, green: 0.91, blue: 0.27, alpha: 1),
    NSColor(calibratedRed: 1.0, green: 0.69, blue: 0.0, alpha: 1),
])!
yellow.draw(in: tile, angle: -52)

NSGraphicsContext.current?.saveGraphicsState()
tile.addClip()
let glow = NSBezierPath(ovalIn: NSRect(x: 350, y: 540, width: 800, height: 650))
NSColor.white.withAlphaComponent(0.24).setFill()
glow.fill()
let lowerShade = NSBezierPath(ovalIn: NSRect(x: -180, y: -270, width: 1250, height: 610))
NSColor(calibratedRed: 0.88, green: 0.43, blue: 0.0, alpha: 0.13).setFill()
lowerShade.fill()
NSGraphicsContext.current?.restoreGraphicsState()

let charcoal = NSColor(calibratedRed: 0.055, green: 0.075, blue: 0.11, alpha: 1)

// Original kangaroo silhouette: tail, body, haunch, feet, neck and head.
let tail = NSBezierPath()
tail.move(to: NSPoint(x: 407, y: 420))
tail.curve(to: NSPoint(x: 99, y: 260), controlPoint1: NSPoint(x: 302, y: 389), controlPoint2: NSPoint(x: 192, y: 316))
tail.curve(to: NSPoint(x: 430, y: 361), controlPoint1: NSPoint(x: 215, y: 292), controlPoint2: NSPoint(x: 326, y: 320))
tail.close()
fill(tail, color: charcoal)

let rearLeg = NSBezierPath()
rearLeg.move(to: NSPoint(x: 430, y: 355))
rearLeg.curve(to: NSPoint(x: 486, y: 212), controlPoint1: NSPoint(x: 397, y: 292), controlPoint2: NSPoint(x: 413, y: 230))
rearLeg.curve(to: NSPoint(x: 720, y: 164), controlPoint1: NSPoint(x: 571, y: 197), controlPoint2: NSPoint(x: 657, y: 181))
rearLeg.curve(to: NSPoint(x: 754, y: 204), controlPoint1: NSPoint(x: 748, y: 173), controlPoint2: NSPoint(x: 765, y: 187))
rearLeg.curve(to: NSPoint(x: 562, y: 291), controlPoint1: NSPoint(x: 695, y: 236), controlPoint2: NSPoint(x: 628, y: 265))
rearLeg.curve(to: NSPoint(x: 430, y: 355), controlPoint1: NSPoint(x: 529, y: 347), controlPoint2: NSPoint(x: 487, y: 371))
rearLeg.close()
fill(rearLeg, color: charcoal)

fill(NSBezierPath(ovalIn: NSRect(x: 350, y: 278, width: 276, height: 360)), color: charcoal)

let neck = NSBezierPath()
neck.move(to: NSPoint(x: 515, y: 581))
neck.curve(to: NSPoint(x: 629, y: 706), controlPoint1: NSPoint(x: 551, y: 639), controlPoint2: NSPoint(x: 575, y: 690))
neck.line(to: NSPoint(x: 709, y: 649))
neck.curve(to: NSPoint(x: 579, y: 520), controlPoint1: NSPoint(x: 655, y: 611), controlPoint2: NSPoint(x: 620, y: 558))
neck.close()
fill(neck, color: charcoal)

fill(NSBezierPath(ovalIn: NSRect(x: 594, y: 645, width: 180, height: 142)), color: charcoal)
let muzzle = NSBezierPath()
muzzle.move(to: NSPoint(x: 714, y: 735))
muzzle.curve(to: NSPoint(x: 839, y: 695), controlPoint1: NSPoint(x: 769, y: 734), controlPoint2: NSPoint(x: 813, y: 719))
muzzle.curve(to: NSPoint(x: 719, y: 666), controlPoint1: NSPoint(x: 811, y: 675), controlPoint2: NSPoint(x: 765, y: 663))
muzzle.close()
fill(muzzle, color: charcoal)

let rearEar = NSBezierPath()
rearEar.move(to: NSPoint(x: 626, y: 750))
rearEar.curve(to: NSPoint(x: 618, y: 920), controlPoint1: NSPoint(x: 607, y: 813), controlPoint2: NSPoint(x: 600, y: 881))
rearEar.curve(to: NSPoint(x: 690, y: 769), controlPoint1: NSPoint(x: 667, y: 872), controlPoint2: NSPoint(x: 687, y: 818))
rearEar.close()
fill(rearEar, color: charcoal)

let frontEar = NSBezierPath()
frontEar.move(to: NSPoint(x: 680, y: 758))
frontEar.curve(to: NSPoint(x: 735, y: 913), controlPoint1: NSPoint(x: 687, y: 821), controlPoint2: NSPoint(x: 708, y: 876))
frontEar.curve(to: NSPoint(x: 745, y: 749), controlPoint1: NSPoint(x: 758, y: 849), controlPoint2: NSPoint(x: 758, y: 795))
frontEar.close()
fill(frontEar, color: charcoal)

let arm = NSBezierPath()
arm.move(to: NSPoint(x: 568, y: 547))
arm.curve(to: NSPoint(x: 708, y: 465), controlPoint1: NSPoint(x: 611, y: 524), controlPoint2: NSPoint(x: 662, y: 482))
arm.lineWidth = 42
arm.lineCapStyle = .round
charcoal.setStroke()
arm.stroke()
fill(NSBezierPath(ovalIn: NSRect(x: 690, y: 445, width: 54, height: 48)), color: charcoal)

// Gemini-inspired four-point spark, rendered as an original gradient mark.
let mainSpark = sparkle(center: NSPoint(x: 494, y: 502), radius: 118, inner: 25)
let sparkShadow = NSShadow()
sparkShadow.shadowColor = NSColor.black.withAlphaComponent(0.24)
sparkShadow.shadowBlurRadius = 20
sparkShadow.shadowOffset = NSSize(width: 0, height: -7)
NSGraphicsContext.current?.saveGraphicsState()
sparkShadow.set()
NSColor.white.setFill()
mainSpark.fill()
NSGraphicsContext.current?.restoreGraphicsState()
let gemini = NSGradient(colors: [
    NSColor(calibratedRed: 0.12, green: 0.50, blue: 1.0, alpha: 1),
    NSColor(calibratedRed: 0.46, green: 0.28, blue: 0.98, alpha: 1),
    NSColor(calibratedRed: 0.95, green: 0.26, blue: 0.68, alpha: 1),
])!
gemini.draw(in: mainSpark, angle: -38)
NSColor.white.withAlphaComponent(0.78).setStroke()
mainSpark.lineWidth = 8
mainSpark.stroke()

let smallSpark = sparkle(center: NSPoint(x: 822, y: 803), radius: 45, inner: 10)
NSColor.white.withAlphaComponent(0.92).setFill()
smallSpark.fill()

NSColor.white.withAlphaComponent(0.52).setStroke()
tile.lineWidth = 8
tile.stroke()

image.unlockFocus()
guard let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fatalError("Unable to render app icon")
}
try png.write(to: URL(fileURLWithPath: output), options: .atomic)
