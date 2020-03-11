package io.textile.threads;

import io.grpc.ManagedChannel;

public interface Config {
    String getSession();
    void setSession(String session);
    ManagedChannel getChannel();
    void init() throws Exception;
}
