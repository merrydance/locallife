import 'package:flutter/material.dart';

class AppColors {
  static const primary = Color(0xFF006B31);
  static const primaryContainer = Color(0xFF038740);
  static const secondary = Color(0xFFFC820C);
  static const tertiary = Color(0xFFB6171E);
  static const surface = Color(0xFFF5FBF1);
  static const surfaceLow = Color(0xFFEFF5EC);
  static const surfaceLowest = Color(0xFFFFFFFF);
  static const onSurface = Color(0xFF171D17);
  static const onSurfaceVariant = Color(0xFF3E4A3F);
  static const outlineVariant = Color(0xFFBDCABB);
  static const warningSoft = Color(0xFFFFF1E3);
  static const dangerSoft = Color(0xFFFDE9E8);
  static const positiveSoft = Color(0xFFE8F5E9);
  static const positive = Color(0xFF2E7D32);
  static const warning = Color(0xFFE65100);
  static const danger = Color(0xFFC62828);
}

class AppSpacing {
  static const xs = 4.0;
  static const sm = 8.0;
  static const md = 12.0;
  static const lg = 16.0;
  static const xl = 24.0;
  static const xxl = 32.0;
}

class AppRadius {
  static const md = 16.0;
  static const lg = 20.0;
  static const xl = 24.0;
  static const xxl = 32.0;
  static const pill = 999.0;
}

class AppTheme {
  static const primaryColor = AppColors.primary;
  static const accentColor = AppColors.secondary;
  static const backgroundColor = AppColors.surface;
  static const cardColor = AppColors.surfaceLowest;
  static const textPrimary = AppColors.onSurface;
  static const textSecondary = AppColors.onSurfaceVariant;

  static ThemeData get lightTheme {
    final base = ThemeData(useMaterial3: true);

    return ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(
        seedColor: primaryColor,
        primary: primaryColor,
        secondary: accentColor,
        tertiary: AppColors.tertiary,
        surface: backgroundColor,
        surfaceContainerLow: AppColors.surfaceLow,
        surfaceContainerLowest: AppColors.surfaceLowest,
        onSurface: textPrimary,
        onSurfaceVariant: textSecondary,
        outlineVariant: AppColors.outlineVariant,
      ),
      scaffoldBackgroundColor: backgroundColor,
      textTheme: base.textTheme.copyWith(
        headlineMedium: base.textTheme.headlineMedium?.copyWith(
          fontSize: 28,
          fontWeight: FontWeight.w700,
          color: textPrimary,
        ),
        titleLarge: base.textTheme.titleLarge?.copyWith(
          fontSize: 22,
          fontWeight: FontWeight.w700,
          color: textPrimary,
        ),
        bodyLarge: base.textTheme.bodyLarge?.copyWith(
          fontSize: 16,
          height: 1.45,
          color: textPrimary,
        ),
        bodyMedium: base.textTheme.bodyMedium?.copyWith(
          fontSize: 14,
          height: 1.4,
          color: textPrimary,
        ),
        labelMedium: base.textTheme.labelMedium?.copyWith(
          fontSize: 12,
          fontWeight: FontWeight.w600,
          color: textSecondary,
        ),
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: AppColors.surface,
        foregroundColor: AppColors.onSurface,
        elevation: 0,
        centerTitle: false,
        surfaceTintColor: Colors.transparent,
      ),
      cardTheme: CardThemeData(
        color: cardColor,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.xl),
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: primaryColor,
          foregroundColor: Colors.white,
          minimumSize: const Size(0, 56),
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.xl,
            vertical: AppSpacing.lg,
          ),
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.lg),
          ),
        ),
      ),
      tabBarTheme: const TabBarThemeData(
        dividerColor: Colors.transparent,
        labelColor: AppColors.onSurface,
        unselectedLabelColor: AppColors.onSurfaceVariant,
        indicatorColor: AppColors.primary,
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: AppColors.onSurface,
        contentTextStyle: const TextStyle(color: Colors.white),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.md),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: AppColors.surfaceLowest,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: AppSpacing.lg,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.lg),
          borderSide: BorderSide.none,
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.lg),
          borderSide: BorderSide(color: Colors.grey.shade300),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.lg),
          borderSide: const BorderSide(color: primaryColor),
        ),
      ),
    );
  }
}
