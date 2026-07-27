package com.github.lingbohuang.autocurl;

import com.intellij.execution.executors.DefaultDebugExecutor;
import com.intellij.execution.executors.DefaultRunExecutor;
import com.intellij.openapi.project.DumbAware;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.ide.CopyPasteManager;
import com.intellij.openapi.ui.Messages;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowFactory;
import com.intellij.ui.JBSplitter;
import com.intellij.ui.components.JBList;
import com.intellij.ui.components.JBScrollPane;
import com.intellij.ui.components.JBTextArea;
import com.intellij.ui.content.Content;
import com.intellij.ui.content.ContentFactory;
import com.intellij.util.ui.JBUI;
import org.jetbrains.annotations.NotNull;

import javax.swing.DefaultListModel;
import javax.swing.JButton;
import javax.swing.JLabel;
import javax.swing.JPanel;
import java.awt.BorderLayout;
import java.awt.FlowLayout;
import java.awt.datatransfer.StringSelection;

public final class AutocurlToolWindowFactory implements ToolWindowFactory, DumbAware {
    @Override
    public void createToolWindowContent(@NotNull Project project, @NotNull ToolWindow toolWindow) {
        CaptureSession session = CaptureSession.getInstance(project);
        DefaultListModel<CaptureSession.RequestEvent> model = new DefaultListModel<>();
        session.requests().forEach(model::addElement);
        JBList<CaptureSession.RequestEvent> list = new JBList<>(model);
        JBTextArea curl = new JBTextArea();
        curl.setEditable(false);
        curl.setLineWrap(false);
        curl.setBorder(JBUI.Borders.empty(8));
        list.addListSelectionListener(event -> {
            CaptureSession.RequestEvent selected = list.getSelectedValue();
            curl.setText(selected == null ? "" : selected.curl());
            curl.setCaretPosition(0);
        });
        list.addMouseListener(new java.awt.event.MouseAdapter() {
            @Override
            public void mouseClicked(java.awt.event.MouseEvent event) {
                if (event.getClickCount() == 2) copySelected(list);
            }
        });

        JLabel status = new JLabel();
        JButton run = new JButton("Run Selected");
        run.addActionListener(event -> RunWithAutocurlAction.run(
                project,
                DefaultRunExecutor.getRunExecutorInstance()
        ));
        JButton debug = new JButton("Debug Selected");
        debug.addActionListener(event -> RunWithAutocurlAction.run(
                project,
                DefaultDebugExecutor.getDebugExecutorInstance()
        ));
        JButton stop = new JButton("Stop");
        stop.addActionListener(event -> session.stop());
        JButton copy = new JButton("Copy cURL");
        copy.addActionListener(event -> copySelected(list));
        JButton render = new JButton("Render JSON");
        render.addActionListener(event -> RenderSelectedRequestAction.renderEditorRequest(project));
        JButton clear = new JButton("Clear");
        clear.addActionListener(event -> {
            session.clear();
            model.clear();
            curl.setText("");
        });

        JPanel toolbar = new JPanel(new FlowLayout(FlowLayout.LEFT, 6, 4));
        toolbar.add(run);
        toolbar.add(debug);
        toolbar.add(stop);
        toolbar.add(copy);
        toolbar.add(render);
        toolbar.add(clear);
        toolbar.add(status);

        JBSplitter split = new JBSplitter(false, 0.45f);
        split.setFirstComponent(new JBScrollPane(list));
        split.setSecondComponent(new JBScrollPane(curl));
        JPanel panel = new JPanel(new BorderLayout());
        panel.add(toolbar, BorderLayout.NORTH);
        panel.add(split, BorderLayout.CENTER);

        Runnable updateState = () ->
                status.setText(session.isRunning() ? "● Capturing" : "○ Stopped");
        updateState.run();
        session.addStateListener(updateState);
        session.addRequestListener(request -> {
            model.addElement(request);
            list.setSelectedIndex(model.size() - 1);
            list.ensureIndexIsVisible(model.size() - 1);
        });

        Content content = ContentFactory.getInstance().createContent(panel, "", false);
        toolWindow.getContentManager().addContent(content);
    }

    private static void copySelected(JBList<CaptureSession.RequestEvent> list) {
        CaptureSession.RequestEvent selected = list.getSelectedValue();
        if (selected == null) {
            Messages.showInfoMessage("Select a captured request first.", "Autocurl");
            return;
        }
        CopyPasteManager.getInstance().setContents(new StringSelection(selected.curl()));
    }
}
