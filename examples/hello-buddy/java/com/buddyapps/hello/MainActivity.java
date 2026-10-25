package com.buddyapps.hello;

import android.app.Activity;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.Bundle;
import android.view.Gravity;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

/**
 * Ejemplo mínimo con el layout plano estilo android2apk: sin rain.toml, sin
 * AndroidX y con la interfaz construida en Java.
 *
 * El diseño imita un grid tipo Bootstrap con LinearLayout: una "página" con
 * fondo gris claro, una tarjeta centrada y filas de dos columnas con
 * layout_weight y gutters.
 */
public class MainActivity extends Activity {
    private static final int FONDO = Color.parseColor("#f8f9fa");
    private static final int TARJETA = Color.WHITE;
    private static final int TEXTO = Color.parseColor("#212529");
    private static final int TEXTO_SUAVE = Color.parseColor("#6c757d");
    private static final int PRIMARIO = Color.parseColor("#0d6efd");
    private static final int SECUNDARIO = Color.parseColor("#6c757d");
    private static final int EXITO = Color.parseColor("#198754");
    private static final int BORDE = Color.parseColor("#dee2e6");
    private static final int CHIP = Color.parseColor("#e9ecef");

    private int pulsaciones = 0;
    private TextView contador;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // "container": fondo de página y padding exterior.
        LinearLayout pagina = new LinearLayout(this);
        pagina.setOrientation(LinearLayout.VERTICAL);
        pagina.setBackgroundColor(FONDO);
        pagina.setPadding(dp(20), dp(24), dp(20), dp(24));

        // "card": superficie blanca con esquinas redondeadas y sombra.
        LinearLayout tarjeta = new LinearLayout(this);
        tarjeta.setOrientation(LinearLayout.VERTICAL);
        tarjeta.setPadding(dp(24), dp(24), dp(24), dp(24));
        tarjeta.setBackground(redondeado(TARJETA, dp(16), 0, 0));
        tarjeta.setElevation(dp(4));

        TextView titulo = new TextView(this);
        titulo.setText("Hola desde Rain");
        titulo.setTextSize(24);
        titulo.setTextColor(TEXTO);
        titulo.setTypeface(Typeface.DEFAULT_BOLD);

        TextView subtitulo = new TextView(this);
        subtitulo.setText("Ejemplo con layout plano y UI en Java");
        subtitulo.setTextSize(14);
        subtitulo.setTextColor(TEXTO_SUAVE);
        subtitulo.setPadding(0, dp(4), 0, dp(20));

        // Contador como "badge" o chip.
        contador = new TextView(this);
        contador.setText("Pulsaciones: 0");
        contador.setTextSize(16);
        contador.setTextColor(PRIMARIO);
        contador.setGravity(Gravity.CENTER);
        contador.setPadding(dp(12), dp(8), dp(12), dp(8));
        contador.setBackground(redondeado(CHIP, dp(999), 0, 0));
        LinearLayout.LayoutParams chipParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT,
                ViewGroup.LayoutParams.WRAP_CONTENT);
        chipParams.gravity = Gravity.START;

        tarjeta.addView(titulo);
        tarjeta.addView(subtitulo);
        tarjeta.addView(contador, chipParams);

        Button restar = boton("−1", TARJETA, PRIMARIO, BORDE);
        Button sumar = boton("+1", PRIMARIO, Color.WHITE, 0);
        Button reiniciar = boton("Reiniciar", SECUNDARIO, Color.WHITE, 0);
        Button doblar = boton("×2", EXITO, Color.WHITE, 0);

        restar.setOnClickListener(v -> cambiar(-1));
        sumar.setOnClickListener(v -> cambiar(1));
        reiniciar.setOnClickListener(v -> {
            pulsaciones = 0;
            actualizarContador();
        });
        doblar.setOnClickListener(v -> {
            pulsaciones *= 2;
            actualizarContador();
        });

        // Dos filas de 2 columnas: col-6 / col-6 con gutter de 12 dp.
        tarjeta.addView(fila(dp(12), restar, sumar), paramsFila(dp(12)));
        tarjeta.addView(fila(dp(12), reiniciar, doblar), paramsFila(0));

        pagina.addView(tarjeta, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT));

        setContentView(pagina);
    }

    private void cambiar(int delta) {
        pulsaciones = Math.max(0, pulsaciones + delta);
        actualizarContador();
    }

    private void actualizarContador() {
        contador.setText("Pulsaciones: " + pulsaciones);
    }

    /** Crea una fila horizontal con columnas de igual peso y gutter entre ellas. */
    private LinearLayout fila(int gutter, Button... botones) {
        LinearLayout fila = new LinearLayout(this);
        fila.setOrientation(LinearLayout.HORIZONTAL);
        fila.setWeightSum(botones.length);
        for (int i = 0; i < botones.length; i++) {
            LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(
                    0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f);
            // Sin gutter en los bordes: solo separación entre columnas.
            lp.setMarginStart(i == 0 ? 0 : gutter / 2);
            lp.setMarginEnd(i == botones.length - 1 ? 0 : gutter / 2);
            fila.addView(botones[i], lp);
        }
        return fila;
    }

    private LinearLayout.LayoutParams paramsFila(int margenInferior) {
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT);
        lp.bottomMargin = margenInferior;
        return lp;
    }

    /** Botón con fondo redondeado; con borde dibuja un botón "outline". */
    private Button boton(String texto, int fondo, int colorTexto, int colorBorde) {
        Button b = new Button(this);
        b.setText(texto);
        b.setTextSize(16);
        b.setTextColor(colorTexto);
        b.setAllCaps(false);
        b.setMinWidth(0);
        b.setMinimumWidth(0);
        b.setMinHeight(dp(48));
        b.setMinimumHeight(dp(48));
        b.setPadding(dp(12), 0, dp(12), 0);
        b.setBackground(redondeado(fondo, dp(12), colorBorde == 0 ? 0 : dp(2), colorBorde));
        return b;
    }

    private GradientDrawable redondeado(int fondo, int radio, int grosor, int colorBorde) {
        GradientDrawable d = new GradientDrawable();
        d.setShape(GradientDrawable.RECTANGLE);
        d.setColor(fondo);
        d.setCornerRadius(radio);
        if (grosor > 0) {
            d.setStroke(grosor, colorBorde);
        }
        return d;
    }

    private int dp(int valor) {
        return Math.round(valor * getResources().getDisplayMetrics().density);
    }
}
