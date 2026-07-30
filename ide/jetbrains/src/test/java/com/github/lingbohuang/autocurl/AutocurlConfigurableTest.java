package com.github.lingbohuang.autocurl;

import com.intellij.ui.components.JBTextArea;
import org.junit.jupiter.api.Test;

import javax.swing.JComponent;
import javax.swing.JScrollPane;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;

class AutocurlConfigurableTest {
    @Test
    void multilineSettingsHaveAVisibleScrollableInputSurface() {
        JBTextArea textArea = new JBTextArea(3, 40);

        JComponent input = AutocurlConfigurable.multilineInput(textArea);

        assertTrue(input instanceof JScrollPane);
        JScrollPane scrollPane = (JScrollPane) input;
        assertSame(textArea, scrollPane.getViewport().getView());
        assertNotNull(scrollPane.getBorder());
        assertTrue(textArea.getLineWrap());
        assertTrue(textArea.getWrapStyleWord());
    }
}
