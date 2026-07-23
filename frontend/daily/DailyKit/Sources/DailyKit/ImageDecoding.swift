import Foundation
import ImageIO

#if canImport(UIKit)
import UIKit

/// `UIImage(data:)` defers decoding to first draw, at the image's full
/// resolution — a cover image from an external source can easily be a few
/// megapixels, so the first draw at a new size (e.g. switching widget size
/// families) stalls the main thread. `CGImageSourceCreateThumbnailAtIndex`
/// decodes straight to a bounded pixel size instead of decoding full-size
/// and scaling after, so callers should always downsample display images
/// through here rather than constructing `UIImage(data:)` directly.
public enum ImageDecoding {
    public static func downsampledImage(from data: Data, maxDimension: CGFloat) -> UIImage? {
        let sourceOptions = [kCGImageSourceShouldCache: false] as CFDictionary
        guard let source = CGImageSourceCreateWithData(data as CFData, sourceOptions) else {
            return nil
        }
        let thumbnailOptions = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceShouldCacheImmediately: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxDimension,
        ] as CFDictionary
        guard let cgImage = CGImageSourceCreateThumbnailAtIndex(source, 0, thumbnailOptions) else {
            return nil
        }
        return UIImage(cgImage: cgImage)
    }
}
#endif
