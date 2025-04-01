# Используем официальный образ nginx
FROM nginx:1.25-alpine

# Удаляем стандартную конфигурацию nginx
RUN rm /etc/nginx/conf.d/default.conf

# Добавляем нашу конфигурацию
COPY docker/nginx/nginx.conf /etc/nginx/conf.d/

# Создаем директорию для статических файлов
RUN mkdir -p /var/www/static

# Копируем статические файлы (если необходимо)
COPY web/static /var/www/static

# Expose порты
EXPOSE 80 443

# Команда для запуска nginx на переднем плане (не как демон)
CMD ["nginx", "-g", "daemon off;"]