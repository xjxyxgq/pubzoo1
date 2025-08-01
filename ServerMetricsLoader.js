import React, { useState, useEffect, useRef } from 'react';
import { Upload, Button, Card, Typography, Progress, Alert, Spin, Icon, message, Modal } from 'antd';
import apiClient from '../../services/apiClient';

const { Title, Paragraph, Text } = Typography;
const { Dragger } = Upload;

// 进度更新间隔时间（毫秒）
const PROGRESS_INTERVAL = 1000;
// 错误重试次数
const MAX_ERROR_RETRIES = 3;

const ServerMetricsLoader = () => {
  const [loading, setLoading] = useState(false);
  const [fileList, setFileList] = useState([]);
  const [uploadResult, setUploadResult] = useState(null);
  const [progress, setProgress] = useState(0);
  const [progressMessage, setProgressMessage] = useState('');
  const [error, setError] = useState(null);
  const [retryCount, setRetryCount] = useState(0);
  const [processingTime, setProcessingTime] = useState(0);
  const progressIntervalRef = useRef(null);
  const processingTimerRef = useRef(null);
  const abortControllerRef = useRef(null);

  // 在组件卸载时清除定时器
  useEffect(() => {
    return () => {
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current);
      }
      if (processingTimerRef.current) {
        clearInterval(processingTimerRef.current);
      }
    };
  }, []);

  // 处理文件上传前的验证
  const beforeUpload = (file) => {
    const isCsv = file.type === 'text/csv' || file.name.endsWith('.csv');
    if (!isCsv) {
      message.error('只能上传CSV文件!');
      return false;
    }
    
    const isLt10M = file.size / 1024 / 1024 < 10;
    if (!isLt10M) {
      message.error('文件大小不能超过10MB!');
      return false;
    }
    
    // 更新文件列表
    setFileList([file]);
    return false; // 阻止自动上传
  };

  // 处理上传文件
  const handleUpload = async () => {
    if (fileList.length === 0) {
      message.error('请先选择一个CSV文件!');
      return;
    }

    setLoading(true);
    setError(null);
    setUploadResult(null);
    setProgress(0);
    setProcessingTime(0);
    setProgressMessage('正在上传文件...');
    setRetryCount(0);

    // 创建取消控制器
    abortControllerRef.current = new AbortController();

    // 创建处理时间计时器
    processingTimerRef.current = setInterval(() => {
      setProcessingTime(prevTime => prevTime + 1);
    }, 1000);

    // 创建FormData对象
    const formData = new FormData();
    formData.append('file', fileList[0]);

    try {
      // 上传文件
      const response = await apiClient.post('/api/cmdb/v1/load-server-metrics-csv', formData, {
        validateStatus: function (status) {
          // 返回所有状态码，这样我们可以处理错误响应
          return true;
        },
        signal: abortControllerRef.current.signal
      });
      
      // 检查HTTP状态码
      if (response.status !== 200) {
        console.error('服务器返回错误状态码:', response.status);
        // 尝试从错误响应中提取错误信息
        if (response.data) {
          const errorMessage = response.data.content || response.data.message || `服务器错误 (${response.status})`;
          const errorDetail = response.data.error_detail || response.data.errorDetail || '';
          const errorLine = response.data.error_line || response.data.errorLine || 0;
          handleError(errorMessage, errorDetail, errorLine);
        } else {
          handleError(`服务器错误 (${response.status})`, '无法连接到服务器或服务器内部错误');
        }
        return;
      }
      
      // 处理正常响应
      if (response.data.success) {
        message.success('文件上传成功，开始处理数据');
        // 更新初始进度
        if (response.data.progress_percentage !== undefined) {
          setProgress(response.data.progress_percentage);
        }
        if (response.data.message) {
          setProgressMessage(response.data.message);
        }
        // 启动进度查询定时器
        startProgressPolling();
      } else {
        // 处理错误响应，优先使用content字段的信息
        const errorMessage = response.data.content || response.data.message || '上传失败';
        const errorDetail = response.data.error_detail || '';
        const errorLine = response.data.error_line || 0;
        handleError(errorMessage, errorDetail, errorLine);
      }
    } catch (error) {
      // 处理网络错误或服务器错误
      console.error('上传错误:', error);
      
      // 检查是否是用户取消的请求
      if (abortControllerRef.current && abortControllerRef.current.signal.aborted) {
        handleError('上传已取消', '用户手动取消了操作');
        return;
      }
      
      // 尝试从响应中提取详细错误信息
      if (error.response && error.response.data) {
        const errorData = error.response.data;
        const errorMessage = errorData.content || errorData.message || '上传文件失败';
        const errorDetail = errorData.error_detail || error.message || '';
        const errorLine = errorData.error_line || 0;
        handleError(errorMessage, errorDetail, errorLine);
      } else {
        handleError('上传文件失败', error.message || '网络错误或服务不可用');
      }
    }
  };

  // 定时查询进度
  const startProgressPolling = () => {
    // 清除可能存在的定时器
    if (progressIntervalRef.current) {
      clearInterval(progressIntervalRef.current);
    }

    // 重置重试计数
    setRetryCount(0);

    // 创建新的定时器
    progressIntervalRef.current = setInterval(async () => {
      try {
        const response = await apiClient.axiosGet('/api/cmdb/v1/server-metrics-csv-progress', {
          signal: abortControllerRef.current?.signal
        });
        const progressData = response.data;

        // 重置重试计数
        setRetryCount(0);

        // 更新进度状态
        setProgress(progressData.progress_percentage || 0);
        setProgressMessage(progressData.message || '处理中...');

        if (!progressData.success) {
          // 处理失败，优先使用content字段信息
          const errorMessage = progressData.content || progressData.message || '处理失败';
          const errorDetail = progressData.error_detail || '';
          const errorLine = progressData.error_line || 0;
          handleError(errorMessage, errorDetail, errorLine);
          clearInterval(progressIntervalRef.current);
        } else if (progressData.is_completed) {
          // 处理完成
          setUploadResult({
            success: true,
            message: progressData.message,
            totalRows: progressData.total_rows,
            processedRows: progressData.processed_rows
          });
          setLoading(false);
          clearInterval(progressIntervalRef.current);
          if (processingTimerRef.current) {
            clearInterval(processingTimerRef.current);
          }
        }
      } catch (error) {
        // 处理网络错误或服务器错误
        console.error('获取进度错误:', error);
        
        // 检查是否是用户取消的请求
        if (abortControllerRef.current && abortControllerRef.current.signal.aborted) {
          clearInterval(progressIntervalRef.current);
          return;
        }
        
        // 增加重试计数
        const newRetryCount = retryCount + 1;
        setRetryCount(newRetryCount);
        
        // 如果连续失败多次，则报告错误
        if (newRetryCount >= MAX_ERROR_RETRIES) {
          // 尝试从响应中提取详细错误信息
          if (error.response && error.response.data) {
            const errorData = error.response.data;
            const errorMessage = errorData.content || errorData.message || '获取进度信息失败';
            const errorDetail = errorData.error_detail || error.message || '';
            const errorLine = errorData.error_line || 0;
            handleError(errorMessage, errorDetail, errorLine);
          } else {
            handleError('获取进度信息失败', '多次尝试连接服务器失败，请检查网络连接或服务器状态');
          }
          clearInterval(progressIntervalRef.current);
        } else {
          // 少于最大重试次数，继续轮询，只显示警告
          console.warn(`进度查询失败 (${newRetryCount}/${MAX_ERROR_RETRIES})，将重试...`);
        }
      }
    }, PROGRESS_INTERVAL);
  };

  // 处理错误
  const handleError = (message, detail, line) => {
    setError({
      message: message,
      detail: detail,
      line: line
    });
    setLoading(false);
    if (progressIntervalRef.current) {
      clearInterval(progressIntervalRef.current);
    }
    if (processingTimerRef.current) {
      clearInterval(processingTimerRef.current);
    }
  };

  // 重置上传状态
  const handleReset = () => {
    setFileList([]);
    setUploadResult(null);
    setProgress(0);
    setProgressMessage('');
    setError(null);
    setLoading(false);
    setProcessingTime(0);
    if (progressIntervalRef.current) {
      clearInterval(progressIntervalRef.current);
    }
    if (processingTimerRef.current) {
      clearInterval(processingTimerRef.current);
    }
  };

  // 取消上传/处理
  const handleCancel = () => {
    Modal.confirm({
      title: '确认取消',
      content: '确定要取消当前的数据处理操作吗？',
      okText: '确认',
      cancelText: '取消',
      onOk: () => {
        // 取消所有请求
        if (abortControllerRef.current) {
          abortControllerRef.current.abort();
          abortControllerRef.current = null;
        }
        
        // 清除定时器
        if (progressIntervalRef.current) {
          clearInterval(progressIntervalRef.current);
          progressIntervalRef.current = null;
        }
        
        if (processingTimerRef.current) {
          clearInterval(processingTimerRef.current);
          processingTimerRef.current = null;
        }
        
        // 更新状态
        setLoading(false);
        setError({
          message: '操作已取消',
          detail: '用户手动取消了处理操作',
          line: 0
        });
        
        message.info('操作已取消');
      }
    });
  };

  // 格式化处理时间
  const formatProcessingTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <>
      <Card 
        title={
          <div>
            <Icon type="upload" style={{ marginRight: '8px' }} />
            上传服务器监控指标CSV文件
          </div>
        }
        style={{ marginBottom: '16px' }}
      >
        <Paragraph>
          <Text strong>功能说明：</Text>
          <br />
          此功能将从CSV文件中读取服务器监控指标数据，并导入到系统中。
          CSV文件必须包含以下列：hostIP、hostName、MaxCpu、MaxMem、MaxDisk。
        </Paragraph>

        <div style={{ marginBottom: '16px' }}>
          <div style={{ marginBottom: '8px' }}>• 最大允许文件大小：10MB</div>
          <div style={{ marginBottom: '8px' }}>• 仅支持CSV格式</div>
          <div style={{ marginBottom: '8px' }}>• 第一行必须是列标题</div>
          <div style={{ marginBottom: '8px' }}>• MaxCpu、MaxMem、MaxDisk列必须是数值类型</div>
        </div>

        <div style={{ marginTop: '20px' }}>
          <Dragger
            beforeUpload={beforeUpload}
            fileList={fileList}
            onRemove={() => setFileList([])}
            disabled={loading}
            multiple={false}
            showUploadList={{ showRemoveIcon: !loading }}
          >
            <p className="ant-upload-drag-icon">
              <Icon type="inbox" />
            </p>
            <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
            <p className="ant-upload-hint">仅支持单个CSV文件上传，请确保文件格式正确</p>
          </Dragger>
        </div>

        <div style={{ textAlign: 'center', marginTop: '20px' }}>
          <Button
            type="primary"
            onClick={handleUpload}
            disabled={fileList.length === 0 || loading}
            loading={loading && !processingTime}
            style={{ marginRight: '10px' }}
          >
            {loading ? '处理中...' : '开始上传'}
          </Button>
          {loading ? (
            <Button 
              type="danger" 
              onClick={handleCancel}
              style={{ marginRight: '10px' }}
            >
              取消操作
            </Button>
          ) : (
            <Button 
              onClick={handleReset} 
              disabled={loading}
              style={{ marginRight: '10px' }}
            >
              重置
            </Button>
          )}
        </div>
      </Card>

      {loading && (
        <Card>
          <div style={{ padding: '20px' }}>
            <div style={{ textAlign: 'center', marginBottom: '16px' }}>
              <Spin />
              <div style={{ marginTop: '8px' }}>
                <Text>{progressMessage}</Text>
              </div>
              <div style={{ marginTop: '4px' }}>
                <Text type="secondary">处理时间: {formatProcessingTime(processingTime)}</Text>
              </div>
            </div>
            <Progress 
              percent={Math.round(progress)} 
              status={error ? 'exception' : 'active'} 
              showInfo={true}
            />
          </div>
        </Card>
      )}

      {error && (
        <Card style={{ marginTop: '16px' }}>
          <Alert
            type="error"
            message="处理失败"
            description={
              <div>
                <p style={{ fontWeight: 'bold' }}>{error.message}</p>
                {error.detail && (
                  <div style={{ marginTop: '8px', padding: '8px', backgroundColor: '#fff1f0', borderRadius: '4px' }}>
                    <p style={{ fontWeight: 'bold', marginBottom: '4px' }}>错误详情:</p>
                    <p style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{error.detail}</p>
                  </div>
                )}
                {error.line > 0 && <p style={{ marginTop: '8px' }}>错误行号: <strong>{error.line}</strong></p>}
                {processingTime > 0 && <p style={{ marginTop: '8px' }}>处理时长: <strong>{formatProcessingTime(processingTime)}</strong></p>}
              </div>
            }
            showIcon
          />
        </Card>
      )}

      {uploadResult && uploadResult.success && (
        <Card style={{ marginTop: '16px' }}>
          <Alert
            type="success"
            message="处理成功"
            description={
              <div>
                <p>{uploadResult.message}</p>
                <div style={{ marginTop: '12px' }}>
                  <p><strong>处理详情：</strong></p>
                  <div style={{ paddingLeft: '16px' }}>
                    <p>总数据行数：<strong>{uploadResult.totalRows}</strong></p>
                    <p>成功处理行数：<strong>{uploadResult.processedRows}</strong></p>
                    <p>处理时长：<strong>{formatProcessingTime(processingTime)}</strong></p>
                  </div>
                </div>
              </div>
            }
            showIcon
          />
        </Card>
      )}
    </>
  );
};

export default ServerMetricsLoader;