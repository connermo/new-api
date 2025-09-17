#!/usr/bin/env python3
"""
One API to New API Migration Script

Migrates users and tokens from One API to New API.

Usage:
    # Migrate all data from SQLite file:
    python migrate_tokens.py --sqlite-file oneapi.db --db-host localhost --db-port 5432 --db-name newapi --db-user postgres --db-password your_password
    
    # Migrate only tokens:
    python migrate_tokens.py --sqlite-file oneapi.db --tokens-only --db-host localhost --db-port 5432 --db-name newapi --db-user postgres --db-password your_password
    
    # Migrate only users:
    python migrate_tokens.py --sqlite-file oneapi.db --users-only --db-host localhost --db-port 5432 --db-name newapi --db-user postgres --db-password your_password

Requirements:
    pip install psycopg2-binary sqlparse
"""

import argparse
import json
import os
import re
import sqlite3
import subprocess
import sys
from datetime import datetime
from typing import List, Dict, Optional, Any

try:
    import psycopg2
    import psycopg2.extras
    import sqlparse
except ImportError:
    print("Please install required packages: pip install psycopg2-binary sqlparse")
    sys.exit(1)


class TokenMigrator:
    def __init__(self, db_config: Dict[str, Any]):
        self.db_config = db_config
        self.conn = None
        
    def connect_db(self):
        """连接PostgreSQL数据库"""
        try:
            self.conn = psycopg2.connect(
                host=self.db_config['host'],
                port=self.db_config['port'],
                database=self.db_config['database'],
                user=self.db_config['user'],
                password=self.db_config['password']
            )
            print(f"Successfully connected to PostgreSQL database: {self.db_config['database']}")
        except Exception as e:
            print(f"Failed to connect to database: {e}")
            sys.exit(1)
    
    def close_db(self):
        """关闭数据库连接"""
        if self.conn:
            self.conn.close()
    
    def backup_database(self, backup_path: Optional[str] = None) -> str:
        """备份PostgreSQL数据库"""
        if not backup_path:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            backup_path = f"newapi_backup_{timestamp}.sql"
        
        print(f"Creating database backup: {backup_path}")
        
        try:
            # 使用pg_dump进行备份
            cmd = [
                'pg_dump',
                '-h', str(self.db_config['host']),
                '-p', str(self.db_config['port']),
                '-U', self.db_config['user'],
                '-d', self.db_config['database'],
                '-f', backup_path,
                '--verbose',
                '--no-password'  # 使用环境变量PGPASSWORD或.pgpass
            ]
            
            # 设置密码环境变量
            env = os.environ.copy()
            env['PGPASSWORD'] = self.db_config['password']
            
            result = subprocess.run(cmd, env=env, capture_output=True, text=True)
            
            if result.returncode == 0:
                print(f"Database backup completed successfully: {backup_path}")
                return backup_path
            else:
                print(f"Backup failed: {result.stderr}")
                return None
                
        except FileNotFoundError:
            print("pg_dump not found. Please install PostgreSQL client tools.")
            print("Attempting alternative backup method...")
            return self.backup_database_alternative(backup_path)
        except Exception as e:
            print(f"Backup failed: {e}")
            return None
    
    def backup_database_alternative(self, backup_path: str) -> Optional[str]:
        """使用Python备份tokens表数据"""
        try:
            cursor = self.conn.cursor()
            
            with open(backup_path, 'w', encoding='utf-8') as f:
                # 写入备份头信息
                f.write(f"-- New API Database Backup\n")
                f.write(f"-- Created: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
                f.write(f"-- Database: {self.db_config['database']}\n\n")
                
                # 备份tokens表结构
                f.write("-- Tokens Table Structure\n")
                cursor.execute("""
                    SELECT column_name, data_type, is_nullable, column_default
                    FROM information_schema.columns
                    WHERE table_name = 'tokens'
                    ORDER BY ordinal_position
                """)
                
                columns = cursor.fetchall()
                if columns:
                    f.write("CREATE TABLE IF NOT EXISTS tokens (\n")
                    col_defs = []
                    for col in columns:
                        col_name, data_type, is_nullable, default = col
                        col_def = f"    {col_name} {data_type}"
                        if is_nullable == 'NO':
                            col_def += " NOT NULL"
                        if default:
                            col_def += f" DEFAULT {default}"
                        col_defs.append(col_def)
                    f.write(",\n".join(col_defs))
                    f.write("\n);\n\n")
                
                # 备份tokens表数据
                f.write("-- Tokens Table Data\n")
                cursor.execute("SELECT COUNT(*) FROM tokens")
                count = cursor.fetchone()[0]
                
                if count > 0:
                    cursor.execute("""
                        SELECT user_id, key, status, name, created_time, accessed_time,
                               expired_time, remain_quota, unlimited_quota, model_limits_enabled,
                               model_limits, allow_ips, used_quota, "group"
                        FROM tokens
                    """)
                    
                    f.write("INSERT INTO tokens (user_id, key, status, name, created_time, accessed_time, expired_time, remain_quota, unlimited_quota, model_limits_enabled, model_limits, allow_ips, used_quota, \"group\") VALUES\n")
                    
                    rows = cursor.fetchall()
                    for i, row in enumerate(rows):
                        # 转换数据为SQL格式
                        values = []
                        for val in row:
                            if val is None:
                                values.append("NULL")
                            elif isinstance(val, str):
                                values.append("'" + val.replace("'", "''") + "'")
                            elif isinstance(val, bool):
                                values.append("true" if val else "false")
                            else:
                                values.append(str(val))
                        
                        f.write(f"({', '.join(values)})")
                        if i < len(rows) - 1:
                            f.write(",\n")
                        else:
                            f.write(";\n")
            
            cursor.close()
            print(f"Alternative backup completed: {backup_path}")
            return backup_path
            
        except Exception as e:
            print(f"Alternative backup failed: {e}")
            return None
    
    def parse_sql_file(self, sql_file: str) -> List[Dict]:
        """解析One API的SQL导出文件，提取tokens表数据"""
        print(f"Parsing SQL file: {sql_file}")
        
        try:
            with open(sql_file, 'r', encoding='utf-8') as f:
                sql_content = f.read()
        except Exception as e:
            print(f"Failed to read SQL file: {e}")
            return []
        
        # 解析SQL语句
        statements = sqlparse.split(sql_content)
        tokens = []
        
        for statement in statements:
            if not statement.strip():
                continue
                
            # 查找tokens表的INSERT语句
            if re.search(r'INSERT INTO.*tokens', statement, re.IGNORECASE):
                extracted_tokens = self.extract_token_data(statement)
                tokens.extend(extracted_tokens)
                print(f"Found {len(extracted_tokens)} tokens in this INSERT statement")
        
        print(f"Total tokens extracted: {len(tokens)}")
        return tokens
    
    def extract_token_data(self, insert_statement: str) -> List[Dict]:
        """从INSERT语句中提取token数据"""
        tokens = []
        
        # 匹配INSERT语句的模式
        insert_pattern = r'INSERT INTO.*tokens.*?VALUES\s*(.+)'
        match = re.search(insert_pattern, insert_statement, re.IGNORECASE | re.DOTALL)
        
        if not match:
            return tokens
        
        values_part = match.group(1)
        
        # 解析VALUES部分，支持多行INSERT
        # 匹配括号内的值: (value1, value2, ...)
        value_pattern = r'\(([^)]+)\)'
        value_matches = re.findall(value_pattern, values_part)
        
        for values_str in value_matches:
            try:
                # 解析单行数据
                token_data = self.parse_token_values(values_str)
                if token_data:
                    tokens.append(token_data)
            except Exception as e:
                print(f"Error parsing token values: {e}")
                continue
        
        return tokens
    
    def parse_token_values(self, values_str: str) -> Optional[Dict]:
        """解析单个token的值字符串"""
        # 移除多余的空格和换行
        values_str = re.sub(r'\s+', ' ', values_str.strip())
        
        # 使用正则表达式分割值，考虑引号内的逗号
        values = []
        current_value = ""
        in_quotes = False
        quote_char = None
        
        i = 0
        while i < len(values_str):
            char = values_str[i]
            
            if not in_quotes and char in ("'", '"'):
                in_quotes = True
                quote_char = char
                current_value += char
            elif in_quotes and char == quote_char:
                # 检查是否是转义引号
                if i + 1 < len(values_str) and values_str[i + 1] == quote_char:
                    current_value += char + char
                    i += 1
                else:
                    in_quotes = False
                    quote_char = None
                    current_value += char
            elif not in_quotes and char == ',':
                values.append(current_value.strip())
                current_value = ""
            else:
                current_value += char
            
            i += 1
        
        # 添加最后一个值
        if current_value.strip():
            values.append(current_value.strip())
        
        # 确保有足够的字段（One API tokens表的字段数）
        if len(values) < 13:  # One API tokens表至少13个字段
            print(f"Warning: Insufficient fields in token data: {len(values)} fields found")
            return None
        
        def clean_value(val: str) -> Any:
            """清理和转换值"""
            val = val.strip()
            if val.upper() == 'NULL':
                return None
            if val.startswith("'") and val.endswith("'"):
                return val[1:-1].replace("''", "'")  # 移除引号并处理转义
            if val.startswith('"') and val.endswith('"'):
                return val[1:-1].replace('""', '"')
            if val.lower() in ('true', 'false'):
                return val.lower() == 'true'
            try:
                if '.' in val:
                    return float(val)
                return int(val)
            except ValueError:
                return val
        
        # 映射到One API的字段结构
        try:
            token = {
                'id': clean_value(values[0]),
                'user_id': clean_value(values[1]),
                'key': clean_value(values[2]),
                'status': clean_value(values[3]),
                'name': clean_value(values[4]),
                'created_time': clean_value(values[5]),
                'accessed_time': clean_value(values[6]),
                'expired_time': clean_value(values[7]),
                'remain_quota': clean_value(values[8]),
                'unlimited_quota': clean_value(values[9]),
                'used_quota': clean_value(values[10]),
                'models': clean_value(values[11]),
                'subnet': clean_value(values[12]) if len(values) > 12 else None,
            }
            return token
        except Exception as e:
            print(f"Error mapping token fields: {e}")
            return None
    
    def convert_to_new_api_format(self, one_api_token: Dict) -> Dict:
        """将One API的token格式转换为New API格式"""
        new_token = {
            'user_id': one_api_token['user_id'],
            'key': one_api_token['key'],
            'status': one_api_token['status'] if one_api_token['status'] is not None else 1,
            'name': one_api_token['name'] or '',
            'created_time': one_api_token['created_time'] or 0,
            'accessed_time': one_api_token['accessed_time'] or 0,
            'expired_time': one_api_token['expired_time'] if one_api_token['expired_time'] is not None else -1,
            'remain_quota': int(one_api_token['remain_quota']) if one_api_token['remain_quota'] is not None else 0,
            'unlimited_quota': bool(one_api_token['unlimited_quota']) if one_api_token['unlimited_quota'] is not None else False,
            'used_quota': int(one_api_token['used_quota']) if one_api_token['used_quota'] is not None else 0,
            'model_limits_enabled': False,
            'model_limits': '',
            'allow_ips': None,
            'group': ''
        }
        
        # 转换模型限制
        if one_api_token.get('models') and one_api_token['models'] != '':
            new_token['model_limits_enabled'] = True
            models_str = str(one_api_token['models'])
            # 将逗号分隔的字符串转换为JSON数组
            models_list = [m.strip() for m in models_str.split(',') if m.strip()]
            if models_list:
                new_token['model_limits'] = json.dumps(models_list)
        
        # 转换IP限制
        if one_api_token.get('subnet') and one_api_token['subnet'] != '':
            new_token['allow_ips'] = str(one_api_token['subnet'])
        
        return new_token
    
    def check_existing_tokens(self) -> set:
        """检查New API数据库中已存在的token keys"""
        cursor = self.conn.cursor()
        try:
            cursor.execute("SELECT key FROM tokens WHERE deleted_at IS NULL")
            existing_keys = {row[0] for row in cursor.fetchall()}
            print(f"Found {len(existing_keys)} existing tokens in New API database")
            return existing_keys
        except Exception as e:
            print(f"Warning: Could not check existing tokens: {e}")
            return set()
        finally:
            cursor.close()
    
    def insert_token(self, token: Dict) -> bool:
        """插入单个token到New API数据库"""
        cursor = self.conn.cursor()
        try:
            insert_sql = """
                INSERT INTO tokens (
                    user_id, key, status, name, created_time, accessed_time, 
                    expired_time, remain_quota, unlimited_quota, model_limits_enabled,
                    model_limits, allow_ips, used_quota, "group"
                ) VALUES (
                    %(user_id)s, %(key)s, %(status)s, %(name)s, %(created_time)s,
                    %(accessed_time)s, %(expired_time)s, %(remain_quota)s, 
                    %(unlimited_quota)s, %(model_limits_enabled)s, %(model_limits)s,
                    %(allow_ips)s, %(used_quota)s, %(group)s
                )
            """
            cursor.execute(insert_sql, token)
            return True
        except Exception as e:
            print(f"Error inserting token '{token.get('name', 'Unknown')}': {e}")
            return False
        finally:
            cursor.close()
    
    def read_from_sqlite(self, sqlite_file: str) -> List[Dict]:
        """从One API的SQLite文件中读取tokens数据"""
        print(f"Reading tokens from SQLite file: {sqlite_file}")
        
        try:
            sqlite_conn = sqlite3.connect(sqlite_file)
            sqlite_conn.row_factory = sqlite3.Row  # 使用字典式访问
            cursor = sqlite_conn.cursor()
            
            # 查询tokens表
            cursor.execute("""
                SELECT id, user_id, key, status, name, created_time, accessed_time,
                       expired_time, remain_quota, unlimited_quota, used_quota, 
                       models, subnet
                FROM tokens
            """)
            
            tokens = []
            for row in cursor.fetchall():
                token = {
                    'id': row['id'],
                    'user_id': row['user_id'],
                    'key': row['key'],
                    'status': row['status'],
                    'name': row['name'],
                    'created_time': row['created_time'],
                    'accessed_time': row['accessed_time'],
                    'expired_time': row['expired_time'],
                    'remain_quota': row['remain_quota'],
                    'unlimited_quota': bool(row['unlimited_quota']),
                    'used_quota': row['used_quota'],
                    'models': row['models'],
                    'subnet': row['subnet']
                }
                tokens.append(token)
            
            sqlite_conn.close()
            print(f"Successfully read {len(tokens)} tokens from SQLite")
            return tokens
            
        except Exception as e:
            print(f"Failed to read from SQLite file: {e}")
            return []

    def read_users_from_sqlite(self, sqlite_file: str) -> List[Dict]:
        """从One API的SQLite文件中读取users数据"""
        print(f"Reading users from SQLite file: {sqlite_file}")
        
        try:
            sqlite_conn = sqlite3.connect(sqlite_file)
            sqlite_conn.row_factory = sqlite3.Row
            cursor = sqlite_conn.cursor()
            
            # 查询users表
            cursor.execute("""
                SELECT id, username, password, display_name, role, status, email,
                       github_id, wechat_id, lark_id, oidc_id, access_token,
                       quota, used_quota, request_count, group_name, aff_code, inviter_id
                FROM users 
                WHERE status != 3
            """)
            
            users = []
            for row in cursor.fetchall():
                user = {
                    'id': row['id'],
                    'username': row['username'],
                    'password': row['password'],
                    'display_name': row['display_name'],
                    'role': row['role'],
                    'status': row['status'],
                    'email': row['email'],
                    'github_id': row['github_id'],
                    'wechat_id': row['wechat_id'],
                    'lark_id': row['lark_id'] if 'lark_id' in row.keys() else None,
                    'oidc_id': row['oidc_id'],
                    'access_token': row['access_token'],
                    'quota': row['quota'],
                    'used_quota': row['used_quota'],
                    'request_count': row['request_count'],
                    'group_name': row['group_name'] if 'group_name' in row.keys() else 'default',
                    'aff_code': row['aff_code'],
                    'inviter_id': row['inviter_id'] if 'inviter_id' in row.keys() else None,
                }
                users.append(user)
            
            sqlite_conn.close()
            print(f"Successfully read {len(users)} users from SQLite")
            return users
            
        except Exception as e:
            print(f"Failed to read users from SQLite file: {e}")
            return []
    
    def convert_user_to_new_api_format(self, one_api_user: Dict) -> Dict:
        """将One API的用户格式转换为New API格式"""
        new_user = {
            'username': one_api_user['username'],
            'password': one_api_user['password'],
            'display_name': one_api_user['display_name'] or one_api_user['username'],
            'role': one_api_user['role'] if one_api_user['role'] is not None else 1,
            'status': one_api_user['status'] if one_api_user['status'] is not None else 1,
            'email': one_api_user['email'] or '',
            'github_id': one_api_user['github_id'] or '',
            'oidc_id': one_api_user['oidc_id'] or '',
            'wechat_id': one_api_user['wechat_id'] or '',
            'telegram_id': '',  # New API has telegram_id, One API doesn't
            'access_token': one_api_user['access_token'],
            'quota': int(one_api_user['quota']) if one_api_user['quota'] is not None else 0,
            'used_quota': int(one_api_user['used_quota']) if one_api_user['used_quota'] is not None else 0,
            'request_count': one_api_user['request_count'] if one_api_user['request_count'] is not None else 0,
            'group': one_api_user['group_name'] or 'default',
            'aff_code': one_api_user['aff_code'] or '',
            'aff_count': 0,  # New API has aff_count, default to 0
        }
        
        return new_user
    
    def check_existing_users(self) -> set:
        """检查New API数据库中已存在的用户名"""
        cursor = self.conn.cursor()
        try:
            cursor.execute("SELECT username FROM users")
            existing_usernames = {row[0] for row in cursor.fetchall()}
            print(f"Found {len(existing_usernames)} existing users in New API database")
            return existing_usernames
        except Exception as e:
            print(f"Warning: Could not check existing users: {e}")
            return set()
        finally:
            cursor.close()
    
    def insert_user(self, user: Dict) -> bool:
        """插入单个用户到New API数据库"""
        cursor = self.conn.cursor()
        try:
            insert_sql = """
                INSERT INTO users (
                    username, password, display_name, role, status, email,
                    github_id, oidc_id, wechat_id, telegram_id, access_token,
                    quota, used_quota, request_count, "group", aff_code, aff_count
                ) VALUES (
                    %(username)s, %(password)s, %(display_name)s, %(role)s, %(status)s,
                    %(email)s, %(github_id)s, %(oidc_id)s, %(wechat_id)s, %(telegram_id)s,
                    %(access_token)s, %(quota)s, %(used_quota)s, %(request_count)s,
                    %(group)s, %(aff_code)s, %(aff_count)s
                )
            """
            cursor.execute(insert_sql, user)
            return True
        except Exception as e:
            print(f"Error inserting user '{user.get('username', 'Unknown')}': {e}")
            return False
        finally:
            cursor.close()

    def migrate_users(self, source_file: str) -> Dict[str, int]:
        """迁移用户数据"""
        print("\n=== 开始迁移用户数据 ===")
        
        # 读取One API用户数据
        one_api_users = self.read_users_from_sqlite(source_file)
        if not one_api_users:
            print("No users found in source database")
            return {'migrated': 0, 'skipped': 0, 'failed': 0}
        
        # 检查已存在的用户
        existing_usernames = self.check_existing_users()
        
        # 开始迁移
        migrated = 0
        skipped = 0
        failed = 0
        
        for user in one_api_users:
            username = user.get('username')
            
            if not username:
                print(f"Skipping user: No username found")
                failed += 1
                continue
            
            if username in existing_usernames:
                print(f"Skipping existing user: {username}")
                skipped += 1
                continue
            
            # 转换格式
            new_user = self.convert_user_to_new_api_format(user)
            
            # 插入数据库
            if self.insert_user(new_user):
                print(f"Migrated user: {username}")
                migrated += 1
            else:
                failed += 1
            
            # 提交事务
            self.conn.commit()
        
        return {'migrated': migrated, 'skipped': skipped, 'failed': failed}

    def migrate_tokens_only(self, source_file: str, source_type: str = 'sql') -> Dict[str, int]:
        """迁移token数据（内部方法）"""
        print("\n=== 开始迁移Token数据 ===")
        
        # 根据源类型获取数据
        if source_type == 'sqlite':
            one_api_tokens = self.read_from_sqlite(source_file)
        else:
            one_api_tokens = self.parse_sql_file(source_file)
        
        if not one_api_tokens:
            print(f"No tokens found in {source_type} file")
            return {'migrated': 0, 'skipped': 0, 'failed': 0}
        
        # 检查已存在的tokens
        existing_keys = self.check_existing_tokens()
        
        # 开始迁移
        migrated = 0
        skipped = 0
        failed = 0
        
        for token in one_api_tokens:
            token_key = token.get('key')
            token_name = token.get('name', 'Unknown')
            
            if not token_key:
                print(f"Skipping token '{token_name}': No key found")
                failed += 1
                continue
            
            if token_key in existing_keys:
                print(f"Skipping existing token: {token_name}")
                skipped += 1
                continue
            
            # 转换格式
            new_token = self.convert_to_new_api_format(token)
            
            # 插入数据库
            if self.insert_token(new_token):
                print(f"Migrated token: {token_name}")
                migrated += 1
            else:
                failed += 1
            
            # 提交事务
            self.conn.commit()
        
        return {'migrated': migrated, 'skipped': skipped, 'failed': failed}

    def migrate_all(self, source_file: str, source_type: str = 'sql', create_backup: bool = True, 
                   backup_path: Optional[str] = None, migrate_users: bool = True, migrate_tokens: bool = True):
        """执行完整迁移"""
        self.connect_db()
        
        try:
            # 创建备份
            backup_file = None
            if create_backup:
                print("Creating database backup before migration...")
                backup_file = self.backup_database(backup_path)
                if backup_file:
                    print(f"Backup created: {backup_file}")
                else:
                    response = input("Backup failed. Continue migration? (y/N): ")
                    if response.lower() != 'y':
                        print("Migration aborted.")
                        return
            
            # 迁移用户数据
            user_stats = {'migrated': 0, 'skipped': 0, 'failed': 0}
            if migrate_users:
                user_stats = self.migrate_users(source_file)
            
            # 迁移token数据
            token_stats = {'migrated': 0, 'skipped': 0, 'failed': 0}
            if migrate_tokens:
                token_stats = self.migrate_tokens_only(source_file, source_type)
            
            # 打印汇总统计信息
            print(f"\n=== 迁移完成汇总 ===")
            if migrate_users:
                print(f"用户迁移:")
                print(f"- 迁移: {user_stats['migrated']} 个用户")
                print(f"- 跳过: {user_stats['skipped']} 个用户 (已存在)")
                print(f"- 失败: {user_stats['failed']} 个用户")
            
            if migrate_tokens:
                print(f"Token迁移:")
                print(f"- 迁移: {token_stats['migrated']} 个tokens")
                print(f"- 跳过: {token_stats['skipped']} 个tokens (已存在)")
                print(f"- 失败: {token_stats['failed']} 个tokens")
            
            total_migrated = user_stats['migrated'] + token_stats['migrated']
            if backup_file and total_migrated > 0:
                print(f"\n数据库备份文件: {backup_file}")
                print("恢复命令: psql -h host -U user -d database -f backup_file.sql")
            
        except Exception as e:
            print(f"Migration failed: {e}")
            if self.conn:
                self.conn.rollback()
        finally:
            self.close_db()


def main():
    parser = argparse.ArgumentParser(description='Migrate users and tokens from One API to New API PostgreSQL')
    
    # Source file options (mutually exclusive)
    source_group = parser.add_mutually_exclusive_group(required=True)
    source_group.add_argument('--sqlite-file', help='Path to One API SQLite database file')
    source_group.add_argument('--sql-file', help='Path to One API SQL export file (tokens only)')
    
    # Migration options (mutually exclusive)
    migration_group = parser.add_mutually_exclusive_group()
    migration_group.add_argument('--users-only', action='store_true', help='Migrate only users (SQLite only)')
    migration_group.add_argument('--tokens-only', action='store_true', help='Migrate only tokens')
    
    # PostgreSQL connection options
    parser.add_argument('--db-host', default='localhost', help='PostgreSQL host')
    parser.add_argument('--db-port', default=5432, type=int, help='PostgreSQL port')
    parser.add_argument('--db-name', required=True, help='PostgreSQL database name')
    parser.add_argument('--db-user', required=True, help='PostgreSQL username')
    parser.add_argument('--db-password', required=True, help='PostgreSQL password')
    
    # Backup options
    parser.add_argument('--no-backup', action='store_true', help='Skip database backup before migration')
    parser.add_argument('--backup-path', help='Custom backup file path')
    
    args = parser.parse_args()
    
    # 验证参数组合
    if args.users_only and args.sql_file:
        print("Error: --users-only can only be used with --sqlite-file")
        sys.exit(1)
    
    db_config = {
        'host': args.db_host,
        'port': args.db_port,
        'database': args.db_name,
        'user': args.db_user,
        'password': args.db_password
    }
    
    migrator = TokenMigrator(db_config)
    
    # 执行迁移
    create_backup = not args.no_backup
    
    if args.sqlite_file:
        source_type = 'sqlite'
        source_file = args.sqlite_file
        
        # 确定迁移类型
        migrate_users = not args.tokens_only
        migrate_tokens = not args.users_only
        
        migrator.migrate_all(source_file, source_type, create_backup, args.backup_path, migrate_users, migrate_tokens)
    
    else:  # SQL file
        source_type = 'sql'
        source_file = args.sql_file
        
        # SQL文件只能迁移tokens
        migrate_users = False
        migrate_tokens = True
        
        migrator.migrate_all(source_file, source_type, create_backup, args.backup_path, migrate_users, migrate_tokens)


if __name__ == '__main__':
    main()