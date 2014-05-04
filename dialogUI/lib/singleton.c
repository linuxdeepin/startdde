#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>

#include <glib.h>
#include <glib/gprintf.h>


static
int binding(int server_sockfd, const char* path)
{
    socklen_t server_len;
    struct sockaddr_un server_addr;

    server_addr.sun_path[0] = '\0'; //make it be an name unix socket
    int path_size = g_sprintf (server_addr.sun_path+1, "%s%d", path, getuid());
    server_addr.sun_family = AF_UNIX;
    server_len = 1 + path_size + offsetof(struct sockaddr_un, sun_path);

    socklen_t val_len = 0;
    // const int reuse = 1;
    // val_len = sizeof reuse;
    // setsockopt(server_sockfd, SOL_SOCKET, SO_REUSEADDR, (const void*)&reuse, val_len);

    // not wait timeout, force quit
    const struct linger linger_val = {1, 0};
    val_len = sizeof linger_val;
    setsockopt(server_sockfd, SOL_SOCKET, SO_LINGER, (const void*)&linger_val, val_len);

    return bind(server_sockfd, (struct sockaddr *)&server_addr, server_len);
}


gboolean is_application_running(const char* path)
{
    int server_sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (0 == binding(server_sockfd, path)) {
        close(server_sockfd);
        return FALSE;
    } else {
        return TRUE;
    }
}


void singleton(const char* name)
{
    static int sd = 0;
    if (sd != 0)
        return;

    int duration = 30;
    sd = socket(AF_UNIX, SOCK_STREAM, 0);
    while (0 != binding(sd, name) && duration < 300) {
        g_debug("binding failed");
        g_usleep(duration);
        duration += 10;
    }
}

