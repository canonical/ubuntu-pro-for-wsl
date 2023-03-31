import 'package:dart_either/dart_either.dart';
import 'package:flutter/foundation.dart';

/// A [ValueNotifier] that holds either a single value [T] or an error [E],
/// never both, with some syntactic sugars.
///
/// Notice that the error type [E] is presented at left, different from Rust,
/// but preserving the underlying [Either](https://pub.dev/packages/dart_either)
/// type, which was designed from Haskell's Either monad.
///
/// > "The Either type is sometimes used to represent a value which is either
///  correct or an error; by convention, the Left constructor is used to hold an
///  error value and the Right constructor is used to hold a correct
///  value (mnemonic: "right" also means "correct")".
class EitherValueNotifier<E, T> extends ValueNotifier<Either<E, T>> {
  /// Creates a [ChangeNotifier] that wraps this value.
  EitherValueNotifier.ok(T val) : super(Right(val));
  EitherValueNotifier.err(E err) : super(Left(err));

  /// Returns an instance of [E] if this holds an error value, null otherwise.
  E? get errorOrNull =>
      value.fold(ifLeft: (error) => error, ifRight: (_) => null);

  /// Returns an instance of [T] if this holds a value, null otherwise.
  T? get valueOrNull => value.orNull();
}
