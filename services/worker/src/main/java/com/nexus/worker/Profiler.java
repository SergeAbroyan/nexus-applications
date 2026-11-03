package com.nexus.worker;

import io.pyroscope.http.Format;
import io.pyroscope.javaagent.EventType;
import io.pyroscope.javaagent.PyroscopeAgent;
import io.pyroscope.javaagent.config.Config;
import org.springframework.beans.factory.DisposableBean;
import org.springframework.beans.factory.InitializingBean;

class Profiler implements InitializingBean, DisposableBean {

    private final String applicationName;
    private final String serverAddress;

    Profiler(String applicationName, String serverAddress) {
        this.applicationName = applicationName;
        this.serverAddress = serverAddress;
    }

    boolean enabled() {
        return !serverAddress.isBlank();
    }

    @Override
    public void afterPropertiesSet() {
        if (enabled()) {
            PyroscopeAgent.start(new Config.Builder()
                    .setApplicationName(applicationName)
                    .setServerAddress(serverAddress)
                    .setProfilingEvent(EventType.ITIMER)
                    .setProfilingAlloc("512k")
                    .setFormat(Format.JFR)
                    .build());
        }
    }

    @Override
    public void destroy() {
        if (enabled() && PyroscopeAgent.isStarted()) {
            PyroscopeAgent.stop();
        }
    }
}
