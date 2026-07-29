package com.github.lingbohuang.autocurl;

import org.junit.jupiter.api.Test;

import javax.swing.JButton;
import javax.swing.JLabel;
import javax.swing.JPanel;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertSame;

class AutocurlToolWindowFactoryTest {
    @Test
    void keepsCopyAndQuickStartInVisibleCompactRows() {
        JButton run = new JButton("Run Selected");
        JButton debug = new JButton("Debug Selected");
        JButton pause = new JButton("Pause Recording");
        JButton stop = new JButton("Stop Session");
        JButton copy = new JButton("Copy cURL");
        JButton render = new JButton("Generate cURL from JSON");
        JButton help = new JButton("Quick Start / 使用说明");
        JButton doctor = new JButton("Environment Check");
        JButton clear = new JButton("Clear");
        JLabel status = new JLabel("○ Stopped");

        JPanel toolbar = AutocurlToolWindowFactory.buildToolbar(
                run,
                debug,
                pause,
                stop,
                copy,
                render,
                help,
                doctor,
                clear,
                status
        );

        assertEquals(4, toolbar.getComponentCount());
        JPanel primaryRow = (JPanel) toolbar.getComponent(0);
        JPanel helpRow = (JPanel) toolbar.getComponent(2);
        assertEquals(3, primaryRow.getComponentCount());
        assertSame(copy, primaryRow.getComponent(2));
        assertEquals(2, helpRow.getComponentCount());
        assertSame(help, helpRow.getComponent(1));
    }
}
