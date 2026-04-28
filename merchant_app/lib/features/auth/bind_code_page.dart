import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

class BindCodePage extends ConsumerStatefulWidget {
  const BindCodePage({super.key});

  @override
  ConsumerState<BindCodePage> createState() => _BindCodePageState();
}

class _BindCodePageState extends ConsumerState<BindCodePage> {
  final _codeController = TextEditingController();
  final _focusNode = FocusNode();
  bool _isFocused = false;

  @override
  void initState() {
    super.initState();
    _focusNode.addListener(() {
      setState(() {
        _isFocused = _focusNode.hasFocus;
      });
      if (_focusNode.hasFocus && _codeController.text == '000000') {
        _codeController.clear();
      }
    });
  }

  @override
  void dispose() {
    _focusNode.dispose();
    _codeController.dispose();
    super.dispose();
  }

  void _handleBind() {
    final code = _codeController.text.trim();
    if (code.length == 6) {
      ref.read(authProvider.notifier).loginWithBindingCode(code);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入6位绑定码')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);

    // Listen for errors
    ref.listen(authProvider, (previous, next) {
      if (next.error != null && next.error != previous?.error) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(next.error!)),
        );
      }
    });

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              Theme.of(context).colorScheme.primary,
              AppColors.primaryContainer,
            ],
          ),
        ),
        child: MerchantContentShell(
          child: Center(
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.xxl),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text(
                      '商户应用绑定',
                      style: TextStyle(
                        fontSize: 24,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    Text(
                      '请输入在微信小程序中生成的 6 位绑定码',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xxl),
                    TextField(
                      controller: _codeController,
                      focusNode: _focusNode,
                      keyboardType: TextInputType.number,
                      maxLength: 6,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        fontSize: 32,
                        letterSpacing: 8,
                        fontWeight: FontWeight.w700,
                      ),
                      onTap: () {
                        if (_codeController.text == '000000') {
                          _codeController.clear();
                        }
                      },
                      decoration: InputDecoration(
                        counterText: '',
                        hintText: _isFocused ? '' : '000000',
                        hintStyle: TextStyle(
                          color: Theme.of(context)
                              .colorScheme
                              .onSurfaceVariant
                              .withValues(alpha: 0.5),
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xl),
                    if (authState.isLoading)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
                        child: CircularProgressIndicator(),
                      )
                    else
                      MerchantPrimaryButton(
                        label: '立即绑定',
                        onPressed: _handleBind,
                      ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
